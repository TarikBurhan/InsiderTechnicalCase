// Package consumer handles messages from the customizer queue: it fills in
// {{variables}} per recipient and forwards the batch to the channel sender.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"notifier/customizer/dto"
	"notifier/customizer/metrics"
	"notifier/customizer/repository"
	"notifier/customizer/template"
	"notifier/pkg/correlation"
	"notifier/pkg/rabbitmq"
)

// Handler personalises queue messages.
type Handler struct {
	contacts   *repository.ContactRepository
	deliveries *repository.DeliveryRepository
	logs       *repository.LogRepository
	publisher  *rabbitmq.Publisher
	log        *slog.Logger
}

// NewHandler creates the handler.
func NewHandler(c *repository.ContactRepository, d *repository.DeliveryRepository, l *repository.LogRepository,
	p *rabbitmq.Publisher, log *slog.Logger) *Handler {
	return &Handler{contacts: c, deliveries: d, logs: l, publisher: p, log: log}
}

// Handle processes one customizer queue message.
func (h *Handler) Handle(ctx context.Context, d amqp.Delivery) error {
	var msg dto.QueueMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		return fmt.Errorf("invalid message: %w", err) // dead-letter
	}
	if correlation.FromContext(ctx) == "" {
		ctx = correlation.WithID(ctx, msg.CorrelationID)
	}
	ctx, span := otel.Tracer("customizer").Start(ctx, "customizer.personalise_batch")
	span.SetAttributes(attribute.String("notification.id", msg.NotificationID), attribute.Int("batch.size", len(msg.Recipients)))
	defer span.End()
	start := time.Now()

	queue, err := rabbitmq.QueueForChannel(msg.Channel)
	if err != nil {
		return err
	}

	ids := make([]int64, 0, len(msg.Recipients))
	for _, r := range msg.Recipients {
		if r.ContactID != nil {
			ids = append(ids, *r.ContactID)
		}
	}
	contacts, err := h.contacts.Load(ctx, ids)
	if err != nil {
		return fmt.Errorf("%w: load contacts: %v", rabbitmq.ErrRequeue, err)
	}

	ok := make([]dto.QueueRecipient, 0, len(msg.Recipients))
	var failed []dto.FailedRecipient
	for _, r := range msg.Recipients {
		if r.ContactID == nil {
			failed = append(failed, dto.FailedRecipient{DeliveryID: r.DeliveryID, Reason: "recipient has no contact to personalise from"})
			continue
		}
		vars, found := contacts[*r.ContactID]
		if !found {
			failed = append(failed, dto.FailedRecipient{DeliveryID: r.DeliveryID, Reason: "contact not found"})
			continue
		}
		rendered := template.RenderContent(msg.Content, vars)
		if err := template.ValidateRendered(msg.Channel, rendered); err != nil {
			failed = append(failed, dto.FailedRecipient{DeliveryID: r.DeliveryID, Reason: err.Error()})
			continue
		}
		r.Content = &rendered
		ok = append(ok, r)
	}

	if err := h.deliveries.MarkFailed(ctx, msg.Channel, failed); err != nil {
		return fmt.Errorf("%w: mark failed: %v", rabbitmq.ErrRequeue, err)
	}

	if len(ok) > 0 {
		msg.Recipients = ok
		body, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if err := h.publisher.Publish(ctx, queue, body, rabbitmq.PriorityValue(msg.Priority)); err != nil {
			return fmt.Errorf("%w: publish: %v", rabbitmq.ErrRequeue, err)
		}
	}

	metrics.RecipientsCustomized.WithLabelValues(msg.Channel, "ok").Add(float64(len(ok)))
	metrics.RecipientsCustomized.WithLabelValues(msg.Channel, "failed").Add(float64(len(failed)))
	metrics.BatchDuration.WithLabelValues(msg.Channel).Observe(time.Since(start).Seconds())

	level := "INFO"
	if len(failed) > 0 {
		level = "WARN"
	}
	if err := h.logs.Write(ctx, msg.NotificationID, msg.Channel, level, "BATCH_CUSTOMIZED",
		fmt.Sprintf("batch %d personalised: %d forwarded to %s, %d failed", msg.BatchNo, len(ok), queue, len(failed)),
		map[string]any{"batch_no": msg.BatchNo, "forwarded": len(ok), "failed": len(failed), "queue": queue}); err != nil {
		h.log.WarnContext(ctx, "could not write log", slog.Any("error", err))
	}
	h.log.InfoContext(ctx, "batch personalised", slog.String("notification_id", msg.NotificationID),
		slog.Int("forwarded", len(ok)), slog.Int("failed", len(failed)))
	return nil
}
