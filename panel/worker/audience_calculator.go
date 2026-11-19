// Package worker contains the Panel's background work: finishing audience
// counts that did not complete within the API timeout.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"notifier/panel/dto"
	"notifier/panel/metrics"
	"notifier/panel/repository"
	"notifier/pkg/correlation"
	"notifier/pkg/healthserver"
	"notifier/pkg/tracing"
)

// LoopName is the watchdog name of the calculator loop.
const LoopName = "panel.audience_calculator"

// Counter counts a validated audience SQL.
type Counter interface {
	CountInBackground(ctx context.Context, sql string, timeout time.Duration) (int64, error)
}

// AudienceCalculatorConfig tunes the calculator.
type AudienceCalculatorConfig struct {
	Workers      int
	PollInterval time.Duration
	Lease        time.Duration // heartbeat lease; expired work is taken over
	Timeout      time.Duration // max duration of one background count
}

// AudienceCalculator finishes AUDIENCE_CALCULATING notifications: it counts
// the audience with a long timeout and moves the notification to NEW (or
// ERROR). Work is claimed with SKIP LOCKED and a heartbeat lease, so several
// Panel instances can share it and a crashed instance's work is resumed.
type AudienceCalculator struct {
	repo     *repository.AudienceCalculationRepository
	counter  Counter
	logs     *repository.LogRepository
	cfg      AudienceCalculatorConfig
	watchdog *healthserver.Watchdog
	log      *slog.Logger

	kick  chan struct{}
	slots chan struct{}
}

// NewAudienceCalculator creates the calculator.
func NewAudienceCalculator(repo *repository.AudienceCalculationRepository, counter Counter, logs *repository.LogRepository,
	cfg AudienceCalculatorConfig, wd *healthserver.Watchdog, log *slog.Logger) *AudienceCalculator {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	return &AudienceCalculator{repo: repo, counter: counter, logs: logs, cfg: cfg, watchdog: wd, log: log,
		kick: make(chan struct{}, 1), slots: make(chan struct{}, cfg.Workers)}
}

// Kick asks the calculator to look for work now instead of at the next tick.
func (c *AudienceCalculator) Kick() {
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

// Run loops until ctx is cancelled.
func (c *AudienceCalculator) Run(ctx context.Context) {
	c.watchdog.Loop(LoopName, c.cfg.PollInterval)
	t := time.NewTicker(c.cfg.PollInterval)
	defer t.Stop()
	for {
		c.poll(ctx)
		c.watchdog.Beat(LoopName)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-c.kick:
		}
	}
}

func (c *AudienceCalculator) poll(ctx context.Context) {
	free := cap(c.slots) - len(c.slots)
	if free == 0 || ctx.Err() != nil {
		return
	}
	jobs, err := c.repo.Claim(ctx, free, c.cfg.Lease)
	if err != nil {
		if ctx.Err() == nil {
			c.log.Error("claiming audience calculations failed", slog.Any("error", err))
		}
		return
	}
	for _, job := range jobs {
		c.slots <- struct{}{}
		go func(job dto.AudienceCalculation) {
			defer func() { <-c.slots }()
			c.calculate(ctx, job)
		}(job)
	}
}

func (c *AudienceCalculator) calculate(ctx context.Context, job dto.AudienceCalculation) {
	// Continue the trace of the API request that created the notification.
	ctx = tracing.FromCarrier(correlation.WithID(ctx, job.CorrelationID), job.TraceContext)
	ctx, span := otel.Tracer("panel").Start(ctx, "panel.calculate_audience")
	span.SetAttributes(attribute.String("notification.id", job.NotificationID))
	defer span.End()

	c.watchdog.JobStarted(job.NotificationID, "audience calculation")
	defer c.watchdog.JobDone(job.NotificationID)

	hbCtx, stop := context.WithCancel(ctx)
	defer stop()
	go c.heartbeat(hbCtx, job.NotificationID)

	start := time.Now()
	log := c.log.With(slog.String("notification_id", job.NotificationID))
	c.writeLog(ctx, job, "INFO", "AUDIENCE_CALCULATION_STARTED", "background audience calculation started", nil)

	count, err := c.counter.CountInBackground(ctx, job.AudienceSQL, c.cfg.Timeout)
	stop()
	elapsed := time.Since(start)

	var pgErr *pgconn.PgError
	switch {
	case err != nil && ctx.Err() != nil:
		// Shutdown: the lease expires and another run picks it up.
		return
	case err != nil && repository.IsTimeout(err):
		c.fail(ctx, job, fmt.Sprintf("audience calculation exceeded %s", c.cfg.Timeout), "timeout")
		return
	case err != nil && errors.As(err, &pgErr):
		c.fail(ctx, job, "audience query rejected by database: "+pgErr.Message, "rejected")
		return
	case err != nil:
		// Connection problems: keep the notification, retry after the lease.
		span.RecordError(err)
		log.WarnContext(ctx, "audience calculation interrupted, will retry", slog.Any("error", err))
		metrics.AudienceCalculations.WithLabelValues("retry").Inc()
		return
	case count == 0:
		c.fail(ctx, job, "audience does not match any master_contact record", "empty")
		return
	}

	done, err := c.repo.Complete(ctx, job.NotificationID, count)
	if err != nil {
		span.RecordError(err)
		log.ErrorContext(ctx, "could not store audience count", slog.Any("error", err))
		return
	}
	if !done {
		log.InfoContext(ctx, "notification left AUDIENCE_CALCULATING meanwhile (cancelled)")
		return
	}
	metrics.AudienceCalculations.WithLabelValues("completed").Inc()
	span.SetAttributes(attribute.Int64("audience.count", count))
	log.InfoContext(ctx, "audience calculated", slog.Int64("count", count), slog.Duration("duration", elapsed))
	c.writeLog(ctx, job, "INFO", "AUDIENCE_CALCULATED", fmt.Sprintf("audience counted: %d contacts; notification is NEW", count),
		map[string]any{"audience_count": count, "duration_ms": elapsed.Milliseconds()})
}

func (c *AudienceCalculator) fail(ctx context.Context, job dto.AudienceCalculation, message, result string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(codes.Error, message)
	if _, err := c.repo.Fail(ctx, job.NotificationID, message); err != nil {
		c.log.ErrorContext(ctx, "could not mark notification as ERROR", slog.Any("error", err))
		return
	}
	metrics.AudienceCalculations.WithLabelValues(result).Inc()
	c.log.WarnContext(ctx, "audience calculation failed", slog.String("notification_id", job.NotificationID), slog.String("reason", message))
	c.writeLog(ctx, job, "ERROR", "AUDIENCE_CALCULATION_FAILED", message, nil)
}

func (c *AudienceCalculator) heartbeat(ctx context.Context, id string) {
	interval := c.cfg.Lease / 3
	if interval <= 0 {
		interval = 10 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.repo.Heartbeat(ctx, id); err != nil && ctx.Err() == nil {
				c.log.WarnContext(ctx, "audience calculation heartbeat failed", slog.Any("error", err))
			}
		}
	}
}

func (c *AudienceCalculator) writeLog(ctx context.Context, job dto.AudienceCalculation, level, event, message string, details any) {
	id := job.NotificationID
	if err := c.logs.Write(ctx, &id, job.Channel, level, event, message, job.CorrelationID, details); err != nil && ctx.Err() == nil {
		c.log.WarnContext(ctx, "could not write notification log", slog.String("event", event), slog.Any("error", err))
	}
}
