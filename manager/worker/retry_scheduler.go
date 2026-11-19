package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"notifier/manager/dto"
	"notifier/manager/metrics"
	"notifier/manager/publisher"
	"notifier/manager/repository"
	"notifier/pkg/correlation"
	"notifier/pkg/tracing"
)

// RetryScheduler re-produces deliveries in RETRY whose next_retry_at has
// passed (senders schedule them ~10 minutes after a failure).
type RetryScheduler struct {
	notifications *repository.NotificationRepository
	deliveries    *repository.DeliveryRepository
	logs          *repository.LogRepository
	publisher     *publisher.BatchPublisher
	interval      time.Duration
	claimLimit    int
	batchSize     int
	log           *slog.Logger
}

// NewRetryScheduler creates the scheduler.
func NewRetryScheduler(n *repository.NotificationRepository, d *repository.DeliveryRepository, l *repository.LogRepository,
	p *publisher.BatchPublisher, interval time.Duration, claimLimit, batchSize int, log *slog.Logger) *RetryScheduler {
	return &RetryScheduler{notifications: n, deliveries: d, logs: l, publisher: p, interval: interval,
		claimLimit: claimLimit, batchSize: batchSize, log: log}
}

// Run loops until ctx is cancelled.
func (s *RetryScheduler) Run(ctx context.Context) {
	runEvery(ctx, s.interval, func() {
		for _, ch := range dto.Channels {
			s.runChannel(ctx, ch)
		}
	})
}

func (s *RetryScheduler) runChannel(ctx context.Context, channel string) {
	for ctx.Err() == nil {
		due, err := s.deliveries.ClaimDueRetries(ctx, channel, s.claimLimit)
		if err != nil {
			s.log.Error("claiming due retries failed", slog.String("channel", channel), slog.Any("error", err))
			return
		}
		if len(due) == 0 {
			return
		}
		s.produce(ctx, channel, due)
		if len(due) < s.claimLimit {
			return
		}
	}
}

func (s *RetryScheduler) produce(ctx context.Context, channel string, due []dto.RetryDelivery) {
	groups := map[string][]dto.RetryDelivery{}
	ids := make([]string, 0)
	for _, d := range due {
		if _, ok := groups[d.NotificationID]; !ok {
			ids = append(ids, d.NotificationID)
		}
		groups[d.NotificationID] = append(groups[d.NotificationID], d)
	}
	notifications, err := s.notifications.Load(ctx, ids)
	if err != nil {
		s.log.Error("loading notifications for retry failed", slog.Any("error", err))
		s.rescheduleAll(ctx, channel, due)
		return
	}

	for nid, items := range groups {
		n, ok := notifications[nid]
		deliveryIDs := idsOf(items)
		if !ok || (n.Status != dto.StatusSending && n.Status != dto.StatusProcessing) {
			// Cancelled / failed notifications: do not retry their deliveries.
			if err := s.deliveries.Cancel(ctx, channel, deliveryIDs); err != nil {
				s.log.Error("cancelling deliveries failed", slog.Any("error", err))
			}
			continue
		}
		s.retryNotification(ctx, channel, n, items)
	}
}

// retryNotification re-produces the due deliveries of one notification as
// part of the notification's original trace.
func (s *RetryScheduler) retryNotification(ctx context.Context, channel string, n dto.Notification, items []dto.RetryDelivery) {
	nctx := tracing.FromCarrier(correlation.WithID(ctx, n.CorrelationID), n.TraceContext)
	nctx, span := otel.Tracer("manager").Start(nctx, "manager.schedule_retry")
	span.SetAttributes(attribute.String("notification.id", n.ID), attribute.String("notification.channel", channel),
		attribute.Int("retry.deliveries", len(items)))
	defer span.End()

	var content dto.MessageContent
	if err := json.Unmarshal(n.Content, &content); err != nil {
		span.RecordError(err)
		s.log.ErrorContext(nctx, "invalid notification content", slog.Any("error", err))
		return
	}
	for _, chunk := range publisher.Chunk(items, s.batchSize) {
		targets := make([]dto.DeliveryTarget, len(chunk))
		attempt := 0
		for i, d := range chunk {
			targets[i] = d.DeliveryTarget
			if d.RetryCount+1 > attempt {
				attempt = d.RetryCount + 1
			}
		}
		if _, err := s.publisher.Publish(nctx, publisher.Batch{Notification: n, Content: content, Attempt: attempt, Targets: targets}); err != nil {
			span.RecordError(err)
			s.log.ErrorContext(nctx, "publishing retry batch failed, rescheduling", slog.Any("error", err))
			_ = s.deliveries.Reschedule(nctx, channel, idsOf(chunk), time.Minute)
			continue
		}
		metrics.RetriesScheduled.WithLabelValues(channel).Add(float64(len(chunk)))
	}
	if err := s.logs.Write(nctx, n.ID, channel, dto.LevelInfo, "RETRY_SCHEDULED",
		fmt.Sprintf("%d deliveries re-produced for retry", len(items)), map[string]any{"count": len(items)}); err != nil {
		s.log.WarnContext(nctx, "could not write log", slog.Any("error", err))
	}
}

func (s *RetryScheduler) rescheduleAll(ctx context.Context, channel string, due []dto.RetryDelivery) {
	if err := s.deliveries.Reschedule(ctx, channel, idsOf(due), time.Minute); err != nil {
		s.log.Error("rescheduling retries failed", slog.Any("error", err))
	}
}

func idsOf(items []dto.RetryDelivery) []int64 {
	out := make([]int64, len(items))
	for i, d := range items {
		out[i] = d.DeliveryID
	}
	return out
}

// runEvery calls fn immediately and then on every tick until ctx ends.
func runEvery(ctx context.Context, interval time.Duration, fn func()) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		fn()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
