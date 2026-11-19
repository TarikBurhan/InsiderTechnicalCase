package worker

import (
	"context"
	"log/slog"
	"time"

	"notifier/manager/dto"
	"notifier/manager/metrics"
	"notifier/manager/repository"
	"notifier/pkg/healthserver"
)

const loopRetention = "manager.retention"

// maxBatchesPerRun bounds one run so a huge backlog is worked off over
// several runs instead of one endless loop.
const maxBatchesPerRun = 1000

// RetentionConfig tunes data retention.
type RetentionConfig struct {
	Days      int           // keep finished notifications this many days (0 = disabled)
	Interval  time.Duration // how often retention runs
	BatchSize int           // rows deleted per statement
}

// Retention deletes notifications (with their deliveries) that finished more
// than Days days ago and audit logs older than Days days. Notifications that
// are still in progress and their logs are never deleted.
type Retention struct {
	repo     *repository.RetentionRepository
	cfg      RetentionConfig
	watchdog *healthserver.Watchdog
	log      *slog.Logger
}

// NewRetention creates the retention worker.
func NewRetention(repo *repository.RetentionRepository, cfg RetentionConfig, wd *healthserver.Watchdog, log *slog.Logger) *Retention {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5000
	}
	return &Retention{repo: repo, cfg: cfg, watchdog: wd, log: log}
}

// Run loops until ctx is cancelled. It does nothing when retention is disabled.
func (r *Retention) Run(ctx context.Context) {
	if r.cfg.Days <= 0 {
		r.log.Info("data retention disabled")
		return
	}
	r.log.Info("data retention enabled", slog.Int("days", r.cfg.Days), slog.Duration("interval", r.cfg.Interval))
	runEvery(ctx, r.cfg.Interval, r.watchdog, loopRetention, func() { r.runOnce(ctx) })
}

func (r *Retention) runOnce(ctx context.Context) {
	start := time.Now()
	deleted := map[string]int64{}
	for _, ch := range dto.Channels {
		// Deliveries first (in small batches), then their notifications, so
		// the notification delete never cascades over a huge table.
		deleted[ch+"_deliveries"] += r.drain(ctx, ch+"_deliveries", func() (int64, error) {
			return r.repo.DeleteDeliveries(ctx, ch, r.cfg.Days, r.cfg.BatchSize)
		})
		deleted["notifications"] += r.drain(ctx, "notifications", func() (int64, error) {
			return r.repo.DeleteNotifications(ctx, ch, r.cfg.Days, r.cfg.BatchSize)
		})
	}
	deleted["notification_logs"] += r.drain(ctx, "notification_logs", func() (int64, error) {
		return r.repo.DeleteLogs(ctx, r.cfg.Days, r.cfg.BatchSize)
	})

	var total int64
	attrs := []any{slog.Int("days", r.cfg.Days), slog.Duration("duration", time.Since(start))}
	for table, n := range deleted {
		total += n
		if n > 0 {
			attrs = append(attrs, slog.Int64(table, n))
		}
	}
	if total > 0 {
		r.log.Info("retention removed old data", attrs...)
	}
}

// drain repeats a batch delete until a batch deletes fewer rows than the
// batch size (nothing left) or the per-run bound is reached.
func (r *Retention) drain(ctx context.Context, table string, deleteBatch func() (int64, error)) int64 {
	var total int64
	for i := 0; i < maxBatchesPerRun && ctx.Err() == nil; i++ {
		n, err := deleteBatch()
		if err != nil {
			if ctx.Err() == nil {
				r.log.Error("retention delete failed", slog.String("table", table), slog.Any("error", err))
			}
			return total
		}
		total += n
		metrics.RetentionDeleted.WithLabelValues(table).Add(float64(n))
		if n < int64(r.cfg.BatchSize) {
			return total
		}
	}
	return total
}
