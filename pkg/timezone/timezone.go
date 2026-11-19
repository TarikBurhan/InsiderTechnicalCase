// Package timezone applies the account time zone, stored in the
// account_settings table, to a service: the Go process (time.Local, so logs
// and API timestamps use it), every PostgreSQL session of its pools, and a
// verification that the database agrees. Scheduled sends and retries are
// stored as absolute instants (timestamptz), the time zone decides how they
// are read, written and displayed.
package timezone

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/pkg/config"
)

// MaxClockDrift is the tolerated difference between service and database clocks.
const MaxClockDrift = 5 * time.Second

// ErrRestartRequired is returned when the account time zone changed while the
// service was running; the service exits and the container restarts with it.
var ErrRestartRequired = errors.New("account time zone changed, restart required")

// Holder keeps the time zone applied to this process and its pools.
type Holder struct {
	mu      sync.RWMutex
	name    string
	changed atomic.Bool
}

// NewHolder creates an empty holder; call Init once the main pool exists.
func NewHolder() *Holder { return &Holder{} }

// Name returns the applied time zone name.
func (h *Holder) Name() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.name
}

// AfterConnect is installed on every pool so each new session uses the
// account time zone.
func (h *Holder) AfterConnect(ctx context.Context, conn *pgx.Conn) error {
	name := h.Name()
	if name == "" {
		return nil
	}
	_, err := conn.Exec(ctx, SetSessionTimezone, name)
	return err
}

// Load reads and validates the account time zone.
func Load(ctx context.Context, pool *pgxpool.Pool) (string, *time.Location, error) {
	var name string
	if err := pool.QueryRow(ctx, LoadAccountTimezone).Scan(&name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, errors.New("account_settings has no 'timezone' key")
		}
		return "", nil, fmt.Errorf("read account timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return "", nil, fmt.Errorf("account timezone %q is not a valid IANA zone: %w", name, err)
	}
	return name, loc, nil
}

// Init loads the account time zone, applies it to the process and to the
// given pools (existing connections are recycled) and verifies it.
func (h *Holder) Init(ctx context.Context, main *pgxpool.Pool, log *slog.Logger, others ...*pgxpool.Pool) error {
	name, loc, err := Load(ctx, main)
	if err != nil {
		return err
	}
	if env := os.Getenv("TZ"); env != "" && env != name {
		log.Warn("container TZ differs from account timezone, account timezone is used",
			slog.String("container_tz", env), slog.String("account_timezone", name))
	}
	h.mu.Lock()
	h.name = name
	h.mu.Unlock()
	time.Local = loc
	for _, p := range append([]*pgxpool.Pool{main}, others...) {
		p.Reset()
	}

	sessionTZ, dbNow, err := h.verify(ctx, main)
	if err != nil {
		return err
	}
	drift := Drift(dbNow)
	if drift > MaxClockDrift {
		log.Warn("service clock differs from database clock", slog.Duration("drift", drift))
	}
	log.Info("account timezone applied", slog.String("timezone", name), slog.String("db_session_timezone", sessionTZ),
		slog.Time("db_now", dbNow), slog.Duration("clock_drift", drift))
	return nil
}

func (h *Holder) verify(ctx context.Context, pool *pgxpool.Pool) (string, time.Time, error) {
	var sessionTZ string
	var dbNow time.Time
	if err := pool.QueryRow(ctx, VerifySession).Scan(&sessionTZ, &dbNow); err != nil {
		return "", time.Time{}, fmt.Errorf("verify session timezone: %w", err)
	}
	if sessionTZ != h.Name() {
		return sessionTZ, dbNow, fmt.Errorf("database session timezone is %q, expected %q", sessionTZ, h.Name())
	}
	return sessionTZ, dbNow, nil
}

// Check is a readiness probe: the session time zone must match the account
// time zone and the clocks must agree.
func (h *Holder) Check(pool *pgxpool.Pool) func(context.Context) error {
	return func(ctx context.Context) error {
		_, dbNow, err := h.verify(ctx, pool)
		if err != nil {
			return err
		}
		if d := Drift(dbNow); d > MaxClockDrift {
			return fmt.Errorf("clock drift with database is %s", d)
		}
		return nil
	}
}

// Watch re-reads the setting every interval. When it changed, onChange is
// called (the service shuts down and its container restarts with the new
// zone); RestartRequired then reports true.
func (h *Holder) Watch(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, onChange func(), log *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		name, _, err := Load(ctx, pool)
		if err != nil {
			if ctx.Err() == nil {
				log.Warn("could not re-read account timezone", slog.Any("error", err))
			}
			continue
		}
		if name != h.Name() {
			log.Warn("account timezone changed, restarting to apply it",
				slog.String("old", h.Name()), slog.String("new", name))
			h.changed.Store(true)
			onChange()
			return
		}
	}
}

// RestartRequired reports whether Watch detected a change.
func (h *Holder) RestartRequired() bool { return h.changed.Load() }

// Drift returns the absolute difference between now and t.
func Drift(t time.Time) time.Duration {
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	return d
}

// WatchInterval is how often services re-read the account time zone
// (TIMEZONE_WATCH_INTERVAL, default 30s).
func WatchInterval() time.Duration {
	return config.Duration("TIMEZONE_WATCH_INTERVAL", 30*time.Second)
}
