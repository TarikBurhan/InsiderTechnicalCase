// Package consumer consumes SMS batches, sends them through the provider
// under the global rate limit and records every outcome.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"notifier/pkg/correlation"
	"notifier/pkg/rabbitmq"
	"notifier/pkg/ratelimit"
	"notifier/smssender/config"
	"notifier/smssender/dto"
	"notifier/smssender/metrics"
	"notifier/smssender/provider"
	"notifier/smssender/repository"
	"notifier/smssender/request"
)

// Handler processes SMS queue messages.
type Handler struct {
	deliveries    *repository.DeliveryRepository
	notifications *repository.NotificationRepository
	logs          *repository.LogRepository
	provider      *provider.Client
	limiter       *ratelimit.Limiter
	cfg           config.Config
	log           *slog.Logger
}

// NewHandler creates the handler.
func NewHandler(d *repository.DeliveryRepository, n *repository.NotificationRepository, l *repository.LogRepository,
	p *provider.Client, limiter *ratelimit.Limiter, cfg config.Config, log *slog.Logger) *Handler {
	return &Handler{deliveries: d, notifications: n, logs: l, provider: p, limiter: limiter, cfg: cfg, log: log}
}

// Handle processes one batch (<= 100 recipients).
func (h *Handler) Handle(ctx context.Context, d amqp.Delivery) error {
	var msg dto.QueueMessage
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		metrics.Batches.WithLabelValues("invalid").Inc()
		return fmt.Errorf("invalid message: %w", err) // dead-letter
	}
	if msg.Channel != config.Channel {
		return fmt.Errorf("unexpected channel %q", msg.Channel)
	}
	if correlation.FromContext(ctx) == "" {
		ctx = correlation.WithID(ctx, msg.CorrelationID)
	}
	ctx, span := otel.Tracer(h.cfg.ServiceName).Start(ctx, h.cfg.ServiceName+".send_batch")
	span.SetAttributes(attribute.String("notification.id", msg.NotificationID), attribute.Int("batch.size", len(msg.Recipients)))
	defer span.End()
	log := h.log.With(slog.String("notification_id", msg.NotificationID), slog.Int("batch_no", msg.BatchNo))

	// 1. Claim: only deliveries still SENDING and not leased are sent.
	ids := make([]int64, len(msg.Recipients))
	for i, r := range msg.Recipients {
		ids[i] = r.DeliveryID
	}
	claimed, err := h.deliveries.Claim(ctx, ids, h.cfg.LeaseDuration)
	if err != nil {
		return fmt.Errorf("%w: claim deliveries: %v", rabbitmq.ErrRequeue, err)
	}
	recipients := make([]dto.QueueRecipient, 0, len(claimed))
	for _, r := range msg.Recipients {
		if claimed[r.DeliveryID] {
			recipients = append(recipients, r)
		}
	}
	if skipped := len(msg.Recipients) - len(recipients); skipped > 0 {
		log.InfoContext(ctx, "skipping deliveries already handled or cancelled", slog.Int("skipped", skipped))
	}
	if len(recipients) == 0 {
		metrics.Batches.WithLabelValues("skipped").Inc()
		return nil
	}

	// 2. Send under the global per-channel rate limit.
	results := h.sendAll(ctx, msg, recipients)

	// 3. Persist outcomes.
	var sent, failed []dto.SendResult
	for _, r := range results {
		if r.Err == nil {
			sent = append(sent, r)
		} else {
			failed = append(failed, r)
		}
	}
	var failedRows []dto.FailedDelivery
	persistErr := withRetry(ctx, func() error {
		if err := h.deliveries.MarkSent(ctx, sent); err != nil {
			return err
		}
		sent = nil // do not mark twice on retry
		rows, err := h.deliveries.MarkFailed(ctx, failed, h.cfg.MaxRetries, h.cfg.RetryDelay)
		if err != nil {
			return err
		}
		failedRows, failed = rows, nil
		return nil
	})
	if persistErr != nil {
		// Messages were already sent; requeueing would not help. The sweeper
		// moves the leased rows back to RETRY once the lease expires.
		log.ErrorContext(ctx, "could not persist delivery results", slog.Any("error", persistErr))
		metrics.Batches.WithLabelValues("persist_failed").Inc()
		return nil
	}

	h.recordOutcome(ctx, msg, results, failedRows)

	// 4. Complete the notification when this was the last pending batch.
	if done, err := h.notifications.CompleteIfDone(ctx, msg.NotificationID); err != nil {
		log.WarnContext(ctx, "completion check failed", slog.Any("error", err))
	} else if done {
		log.InfoContext(ctx, "notification completed")
		_ = h.logs.Write(ctx, dto.LogEntry{NotificationID: msg.NotificationID, Level: "INFO", Event: "COMPLETED", Message: "all deliveries finished"})
	}
	metrics.Batches.WithLabelValues("processed").Inc()
	return nil
}

// sendAll sends recipients in rate-limited waves, each wave concurrently.
func (h *Handler) sendAll(ctx context.Context, msg dto.QueueMessage, recipients []dto.QueueRecipient) []dto.SendResult {
	results := make([]dto.SendResult, 0, len(recipients))
	remaining := recipients
	for len(remaining) > 0 {
		granted, err := h.limiter.Acquire(ctx, len(remaining))
		if err != nil {
			for _, r := range remaining {
				results = append(results, dto.SendResult{DeliveryID: r.DeliveryID, Recipient: r.To, Attempt: msg.AttemptFor(r),
					Err: fmt.Errorf("rate limiter: %w", err)})
			}
			break
		}
		wave := remaining[:granted]
		remaining = remaining[granted:]
		results = append(results, h.sendWave(ctx, msg, wave)...)
	}
	return results
}

func (h *Handler) sendWave(ctx context.Context, msg dto.QueueMessage, wave []dto.QueueRecipient) []dto.SendResult {
	out := make([]dto.SendResult, len(wave))
	sem := make(chan struct{}, max(1, h.cfg.SendConcurrency))
	var wg sync.WaitGroup
	for i, r := range wave {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, r dto.QueueRecipient) {
			defer func() { <-sem; wg.Done() }()
			start := time.Now()
			attempt := msg.AttemptFor(r)
			key := fmt.Sprintf("%s-%d-%d", config.Channel, r.DeliveryID, attempt)
			resp, err := h.provider.Send(ctx, request.NewProviderRequest(r.To, msg.ContentFor(r)), key)
			elapsed := time.Since(start)
			metrics.ProviderLatency.Observe(elapsed.Seconds())
			out[i] = dto.SendResult{
				DeliveryID: r.DeliveryID, Recipient: r.To, ProviderMessageID: resp.MessageID,
				Err: err, Permanent: provider.IsPermanent(err), LatencyMs: elapsed.Milliseconds(), Attempt: attempt,
			}
		}(i, r)
	}
	wg.Wait()
	return out
}

func (h *Handler) recordOutcome(ctx context.Context, msg dto.QueueMessage, results []dto.SendResult, failed []dto.FailedDelivery) {
	statusByID := make(map[int64]dto.FailedDelivery, len(failed))
	retry, perm := 0, 0
	for _, f := range failed {
		statusByID[f.DeliveryID] = f
		if f.Status == dto.DeliveryError {
			perm++
		} else {
			retry++
		}
	}
	sentCount := len(results) - len(failed)
	metrics.Sends.WithLabelValues("sent").Add(float64(sentCount))
	metrics.Sends.WithLabelValues("retry").Add(float64(retry))
	metrics.Sends.WithLabelValues("error").Add(float64(perm))

	entries := make([]dto.LogEntry, 0, len(results)+1)
	if h.cfg.LogEachDelivery {
		for _, r := range results {
			id := r.DeliveryID
			e := dto.LogEntry{NotificationID: msg.NotificationID, DeliveryID: &id}
			if r.Err == nil {
				e.Level, e.Event, e.Message = "INFO", "DELIVERY_SENT", "accepted by provider"
				e.Details, _ = json.Marshal(map[string]any{"provider_message_id": r.ProviderMessageID, "latency_ms": r.LatencyMs, "attempt": r.Attempt})
			} else {
				f := statusByID[r.DeliveryID]
				e.Level, e.Event, e.Message = "WARN", "DELIVERY_RETRY", r.Err.Error()
				if f.Status == dto.DeliveryError {
					e.Level, e.Event = "ERROR", "DELIVERY_FAILED"
				}
				e.Details, _ = json.Marshal(map[string]any{"retry_count": f.RetryCount, "permanent": r.Permanent, "attempt": r.Attempt})
			}
			entries = append(entries, e)
		}
	}
	summary, _ := json.Marshal(map[string]any{"batch_no": msg.BatchNo, "attempt": msg.Attempt, "sent": sentCount, "retry": retry, "error": perm})
	entries = append(entries, dto.LogEntry{
		NotificationID: msg.NotificationID, Level: "INFO", Event: "BATCH_SENT",
		Message: fmt.Sprintf("batch processed: %d sent, %d retry, %d error", sentCount, retry, perm), Details: summary,
	})
	if err := h.logs.WriteMany(ctx, entries); err != nil {
		h.log.WarnContext(ctx, "could not write delivery logs", slog.Any("error", err))
	}
	h.log.InfoContext(ctx, "batch processed", slog.String("notification_id", msg.NotificationID),
		slog.Int("sent", sentCount), slog.Int("retry", retry), slog.Int("error", perm))
}

func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return err
}
