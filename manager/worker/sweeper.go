package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"notifier/manager/dto"
	"notifier/manager/metrics"
	"notifier/manager/repository"
	"notifier/pkg/healthserver"
	"notifier/pkg/tracing"
)

// Sweeper recovers deliveries stuck in SENDING for too long (queue message
// dead-lettered, consumer crashed after claiming, ...). They are moved to
// RETRY without consuming a retry attempt, and the retry scheduler produces
// them again. Senders claim rows before sending, so this cannot cause a
// delivery that is currently being sent to be sent twice.
type Sweeper struct {
	deliveries *repository.DeliveryRepository
	logs       *repository.LogRepository
	interval   time.Duration
	staleAfter time.Duration
	limit      int
	watchdog   *healthserver.Watchdog
	log        *slog.Logger
}

// NewSweeper creates the sweeper.
func NewSweeper(d *repository.DeliveryRepository, l *repository.LogRepository, interval, staleAfter time.Duration, limit int, wd *healthserver.Watchdog, log *slog.Logger) *Sweeper {
	return &Sweeper{deliveries: d, logs: l, interval: interval, staleAfter: staleAfter, limit: limit, watchdog: wd, log: log}
}

// Run loops until ctx is cancelled.
func (s *Sweeper) Run(ctx context.Context) {
	runEvery(ctx, s.interval, s.watchdog, loopSweeper, func() {
		for _, ch := range dto.Channels {
			swept, err := s.deliveries.SweepStale(ctx, ch, s.staleAfter, s.limit)
			if err != nil {
				if ctx.Err() == nil {
					s.log.Error("sweeper failed", slog.String("channel", ch), slog.Any("error", err))
				}
				continue
			}
			if len(swept) == 0 {
				continue
			}
			metrics.StaleSwept.WithLabelValues(ch).Add(float64(len(swept)))
			perNotification := map[string]int{}
			traceOf := map[string]string{}
			for _, d := range swept {
				perNotification[d.NotificationID]++
				traceOf[d.NotificationID] = d.TraceContext
			}
			for nid, count := range perNotification {
				nctx, span := otel.Tracer("manager").Start(tracing.FromCarrier(ctx, traceOf[nid]), "manager.recover_stale_deliveries")
				span.SetAttributes(attribute.String("notification.id", nid), attribute.Int("deliveries", count))
				s.log.WarnContext(nctx, "stale deliveries moved back to RETRY", slog.String("notification_id", nid), slog.Int("count", count))
				_ = s.logs.Write(nctx, nid, ch, dto.LevelWarn, "STALE_DELIVERIES_RECOVERED",
					fmt.Sprintf("%d deliveries stuck in SENDING moved to RETRY", count), map[string]any{"count": count})
				span.End()
			}
		}
	})
}
