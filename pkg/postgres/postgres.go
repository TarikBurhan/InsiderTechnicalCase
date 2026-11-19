// Package postgres creates pgx connection pools. No ORM is used anywhere in
// the project; every service talks to PostgreSQL with plain SQL via pgx.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DSN builds a postgres connection url.
func DSN(host, port, user, password, database, sslMode string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%s", host, port),
		Path:     "/" + database,
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
	}
	return u.String()
}

// Options tunes the pool.
type Options struct {
	MaxConns        int32
	ApplicationName string
	// AfterConnect runs on every new physical connection (used to set the
	// account time zone on each session).
	AfterConnect func(context.Context, *pgx.Conn) error
}

// NewPool creates a pool and waits (with backoff) until the database answers,
// so that services can start before PostgreSQL is fully ready.
func NewPool(ctx context.Context, dsn string, opts Options, log *slog.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	if opts.ApplicationName != "" {
		cfg.ConnConfig.RuntimeParams["application_name"] = opts.ApplicationName
	}
	if opts.AfterConnect != nil {
		cfg.AfterConnect = opts.AfterConnect
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	backoff := 500 * time.Millisecond
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if attempt >= 30 {
			pool.Close()
			return nil, fmt.Errorf("database not reachable: %w", err)
		}
		log.Warn("database not ready, retrying", slog.Int("attempt", attempt), slog.String("error", err.Error()))
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}
