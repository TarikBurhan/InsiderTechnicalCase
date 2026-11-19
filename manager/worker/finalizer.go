package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"notifier/manager/dto"
	"notifier/manager/repository"
	"notifier/pkg/correlation"
	"notifier/pkg/tracing"
)

// Finalizer is the safety net that moves SENDING notifications to COMPLETE
// once every delivery reached a final state (SENT / ERROR / CANCELLED).
// Senders also try this right after each batch; the finalizer covers races.
type Finalizer struct {
	notifications *repository.NotificationRepository
	logs          *repository.LogRepository
	interval      time.Duration
	log           *slog.Logger
}

// NewFinalizer creates the finalizer.
func NewFinalizer(n *repository.NotificationRepository, l *repository.LogRepository, interval time.Duration, log *slog.Logger) *Finalizer {
	return &Finalizer{notifications: n, logs: l, interval: interval, log: log}
}

// Run loops until ctx is cancelled.
func (f *Finalizer) Run(ctx context.Context) {
	runEvery(ctx, f.interval, func() {
		for _, ch := range dto.Channels {
			done, err := f.notifications.CompleteFinished(ctx, ch)
			if err != nil {
				if ctx.Err() == nil {
					f.log.Error("finalizer failed", slog.String("channel", ch), slog.Any("error", err))
				}
				continue
			}
			for _, c := range done {
				cctx := tracing.FromCarrier(correlation.WithID(ctx, c.CorrelationID), c.TraceContext)
				cctx, span := otel.Tracer("manager").Start(cctx, "manager.complete_notification")
				span.SetAttributes(attribute.String("notification.id", c.ID), attribute.String("completed_by", "finalizer"))
				f.log.InfoContext(cctx, "notification completed", slog.String("notification_id", c.ID))
				_ = f.logs.Write(cctx, c.ID, ch, dto.LevelInfo, "COMPLETED", "all deliveries finished", nil)
				span.End()
			}
		}
	})
}
