package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"notifier/manager/dto"
	"notifier/manager/repository"
)

// DispatcherConfig tunes polling and recovery.
type DispatcherConfig struct {
	PollInterval         time.Duration
	Workers              int
	StaleProcessingAfter time.Duration
	RecoveryInterval     time.Duration
}

// Dispatcher polls the database for due NEW notifications (including
// scheduled ones whose scheduled_at has passed) and runs them on a bounded
// pool of concurrent processors. It also recovers PROCESSING notifications
// left behind by a crashed Manager: once at startup and periodically.
type Dispatcher struct {
	notifications *repository.NotificationRepository
	processor     *Processor
	cfg           DispatcherConfig
	log           *slog.Logger

	slots chan struct{}
	wg    sync.WaitGroup
}

// NewDispatcher creates a dispatcher.
func NewDispatcher(n *repository.NotificationRepository, p *Processor, cfg DispatcherConfig, log *slog.Logger) *Dispatcher {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	return &Dispatcher{notifications: n, processor: p, cfg: cfg, log: log, slots: make(chan struct{}, cfg.Workers)}
}

// Run blocks until ctx is cancelled and all in-flight work stopped.
func (d *Dispatcher) Run(ctx context.Context) {
	d.log.Info("dispatcher started", slog.Int("workers", d.cfg.Workers), slog.Duration("poll_interval", d.cfg.PollInterval))

	// Startup recovery: re-process notifications stuck in PROCESSING.
	d.recover(ctx)
	d.poll(ctx)

	poll := time.NewTicker(d.cfg.PollInterval)
	recovery := time.NewTicker(d.cfg.RecoveryInterval)
	defer poll.Stop()
	defer recovery.Stop()
	for {
		select {
		case <-ctx.Done():
			d.wg.Wait()
			d.log.Info("dispatcher stopped")
			return
		case <-poll.C:
			d.poll(ctx)
		case <-recovery.C:
			d.recover(ctx)
		}
	}
}

func (d *Dispatcher) free() int { return cap(d.slots) - len(d.slots) }

// poll claims as many NEW notifications as there are free workers.
func (d *Dispatcher) poll(ctx context.Context) {
	for ctx.Err() == nil {
		free := d.free()
		if free == 0 {
			return
		}
		items, err := d.notifications.ClaimNew(ctx, free)
		if err != nil {
			if ctx.Err() == nil {
				d.log.Error("claiming NEW notifications failed", slog.Any("error", err))
			}
			return
		}
		d.dispatch(ctx, items)
		if len(items) < free {
			return
		}
	}
}

// recover claims PROCESSING notifications whose heartbeat is stale.
func (d *Dispatcher) recover(ctx context.Context) {
	free := d.free()
	if free == 0 || ctx.Err() != nil {
		return
	}
	items, err := d.notifications.ClaimStale(ctx, free, d.cfg.StaleProcessingAfter)
	if err != nil {
		if ctx.Err() == nil {
			d.log.Error("claiming stale PROCESSING notifications failed", slog.Any("error", err))
		}
		return
	}
	if len(items) > 0 {
		d.log.Warn("recovering notifications left in PROCESSING", slog.Int("count", len(items)))
	}
	d.dispatch(ctx, items)
}

func (d *Dispatcher) dispatch(ctx context.Context, items []dto.Notification) {
	for _, n := range items {
		d.slots <- struct{}{}
		d.wg.Add(1)
		go func(n dto.Notification) {
			defer func() {
				<-d.slots
				d.wg.Done()
			}()
			d.processor.Process(ctx, n)
		}(n)
	}
}
