// Package worker contains the Manager's asynchronous loops: the NEW
// notification dispatcher, crash recovery, retry scheduler, finalizer and
// stuck-delivery sweeper. Each runs in its own goroutine and never blocks
// the others.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"notifier/manager/dto"
	"notifier/manager/metrics"
	"notifier/manager/publisher"
	"notifier/manager/repository"
	"notifier/pkg/correlation"
)

var errCancelled = errors.New("notification cancelled")

// permanentError marks failures that will not succeed on retry.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// ProcessorConfig tunes the processor.
type ProcessorConfig struct {
	BatchSize             int
	HeartbeatInterval     time.Duration
	MaxProcessingAttempts int
}

// Processor turns one notification into delivery rows + queue messages.
type Processor struct {
	notifications *repository.NotificationRepository
	deliveries    *repository.DeliveryRepository
	audience      *repository.AudienceRepository
	logs          *repository.LogRepository
	publisher     *publisher.BatchPublisher
	cfg           ProcessorConfig
	log           *slog.Logger
}

// NewProcessor creates a processor.
func NewProcessor(n *repository.NotificationRepository, d *repository.DeliveryRepository, a *repository.AudienceRepository,
	l *repository.LogRepository, p *publisher.BatchPublisher, cfg ProcessorConfig, log *slog.Logger) *Processor {
	return &Processor{notifications: n, deliveries: d, audience: a, logs: l, publisher: p, cfg: cfg, log: log}
}

// Process resolves the audience, writes deliveries batch by batch (<= 100
// recipients each), produces every batch and finally moves the notification
// to SENDING. Transient failures leave it in PROCESSING so the recovery loop
// picks it up again; permanent failures move it to ERROR.
func (p *Processor) Process(ctx context.Context, n dto.Notification) {
	ctx = correlation.WithID(ctx, n.CorrelationID)
	ctx, span := otel.Tracer("manager").Start(ctx, "manager.process_notification")
	span.SetAttributes(attribute.String("notification.id", n.ID), attribute.String("notification.channel", n.Channel))
	defer span.End()

	metrics.InFlight.Inc()
	defer metrics.InFlight.Dec()
	start := time.Now()

	hbCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go p.heartbeat(hbCtx, n.ID)

	log := p.log.With(slog.String("notification_id", n.ID), slog.String("channel", n.Channel))
	log.InfoContext(ctx, "processing notification", slog.Int("attempt", n.ProcessingAttempts))
	p.writeLog(ctx, n, dto.LevelInfo, "PROCESSING_STARTED", "manager started processing",
		map[string]any{"attempt": n.ProcessingAttempts})

	total, batches, err := p.produce(ctx, n)
	stopHeartbeat()
	metrics.ProcessingDuration.WithLabelValues(n.Channel).Observe(time.Since(start).Seconds())

	var perm permanentError
	switch {
	case errors.Is(err, errCancelled):
		log.InfoContext(ctx, "notification cancelled during processing")
		p.writeLog(ctx, n, dto.LevelInfo, "PROCESSING_CANCELLED", "notification was cancelled while processing", nil)
		metrics.NotificationsProcessed.WithLabelValues(n.Channel, "cancelled").Inc()
		return

	case err != nil && ctx.Err() != nil:
		log.WarnContext(ctx, "processing interrupted by shutdown, will be recovered")
		return

	case errors.As(err, &perm) || (err != nil && n.ProcessingAttempts >= p.cfg.MaxProcessingAttempts):
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "processing failed permanently", slog.Any("error", err))
		bg := context.WithoutCancel(ctx)
		if mErr := p.notifications.MarkError(bg, n.ID, err.Error()); mErr != nil {
			log.ErrorContext(ctx, "could not mark notification as ERROR", slog.Any("error", mErr))
		}
		p.writeLog(bg, n, dto.LevelError, "PROCESSING_FAILED", err.Error(), map[string]any{"attempt": n.ProcessingAttempts})
		metrics.NotificationsProcessed.WithLabelValues(n.Channel, "error").Inc()
		return

	case err != nil:
		span.RecordError(err)
		log.WarnContext(ctx, "processing failed, will be retried by recovery", slog.Any("error", err))
		p.writeLog(ctx, n, dto.LevelWarn, "PROCESSING_INTERRUPTED", err.Error(), map[string]any{"attempt": n.ProcessingAttempts})
		metrics.NotificationsProcessed.WithLabelValues(n.Channel, "retry").Inc()
		return
	}

	if _, err := p.notifications.MarkSending(ctx, n.ID, total); err != nil {
		log.ErrorContext(ctx, "could not mark notification as SENDING", slog.Any("error", err))
		return
	}
	p.writeLog(ctx, n, dto.LevelInfo, "PRODUCED", fmt.Sprintf("%d recipients produced in %d batches", total, batches),
		map[string]any{"total_recipients": total, "batches": batches, "duration_ms": time.Since(start).Milliseconds()})
	metrics.NotificationsProcessed.WithLabelValues(n.Channel, "produced").Inc()
	log.InfoContext(ctx, "notification produced", slog.Int("recipients", total), slog.Int("batches", batches))

	// Empty audiences or batches that were already delivered complete now.
	if done, err := p.notifications.CompleteIfDone(ctx, n.Channel, n.ID); err == nil && done {
		p.writeLog(ctx, n, dto.LevelInfo, "COMPLETED", "all deliveries finished", nil)
	}
}

func (p *Processor) produce(ctx context.Context, n dto.Notification) (total, batches int, err error) {
	var content dto.MessageContent
	if err := json.Unmarshal(n.Content, &content); err != nil {
		return 0, 0, permanentError{fmt.Errorf("invalid content: %w", err)}
	}

	flush := func(contacts []dto.AudienceContact) error {
		status, err := p.notifications.Status(ctx, n.ID)
		if err != nil {
			return err
		}
		if status == dto.StatusCancelled {
			return errCancelled
		}
		batches++
		targets, err := p.deliveries.InsertBatch(ctx, n.Channel, n.ID, batches, contacts)
		if err != nil {
			return fmt.Errorf("insert deliveries: %w", err)
		}
		total += len(contacts)
		if len(targets) == 0 {
			return nil
		}
		queue, err := p.publisher.Publish(ctx, publisher.Batch{
			Notification: n, Content: content, BatchNo: batches, Attempt: 1, Targets: targets,
		})
		if err != nil {
			return fmt.Errorf("publish batch %d: %w", batches, err)
		}
		p.writeLog(ctx, n, dto.LevelInfo, "BATCH_PRODUCED", fmt.Sprintf("batch %d produced to %s", batches, queue),
			map[string]any{"batch_no": batches, "size": len(targets), "queue": queue})
		return nil
	}

	if n.AudienceType == dto.AudienceTypeDirect {
		var recipients []string
		if err := json.Unmarshal(n.Recipients, &recipients); err != nil {
			return 0, 0, permanentError{fmt.Errorf("invalid recipients: %w", err)}
		}
		for _, chunk := range publisher.Chunk(recipients, p.cfg.BatchSize) {
			contacts := make([]dto.AudienceContact, len(chunk))
			for i, r := range chunk {
				contacts[i] = dto.AudienceContact{Address: r}
			}
			if err := flush(contacts); err != nil {
				return total, batches, err
			}
		}
		return total, batches, nil
	}

	if n.AudienceSQL == nil {
		return 0, 0, permanentError{errors.New("notification has no audience sql")}
	}
	err = p.audience.Stream(ctx, *n.AudienceSQL, n.Channel, p.cfg.BatchSize, flush)
	var qErr *repository.AudienceQueryError
	if errors.As(err, &qErr) {
		return total, batches, permanentError{err}
	}
	return total, batches, err
}

func (p *Processor) heartbeat(ctx context.Context, id string) {
	t := time.NewTicker(p.cfg.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := p.notifications.Heartbeat(ctx, id); err != nil && ctx.Err() == nil {
				p.log.WarnContext(ctx, "heartbeat failed", slog.String("notification_id", id), slog.Any("error", err))
			}
		}
	}
}

func (p *Processor) writeLog(ctx context.Context, n dto.Notification, level, event, message string, details any) {
	if err := p.logs.Write(ctx, n.ID, n.Channel, level, event, message, details); err != nil && ctx.Err() == nil {
		p.log.WarnContext(ctx, "could not write notification log", slog.String("event", event), slog.Any("error", err))
	}
}
