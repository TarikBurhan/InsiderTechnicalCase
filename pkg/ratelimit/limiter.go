// Package ratelimit implements a distributed fixed-window rate limiter backed
// by PostgreSQL. Because the counter lives in the database, the limit
// (e.g. 100 messages / second / channel) holds no matter how many sender
// replicas are running.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"

	"notifier/pkg/tracing"
)

// Limiter grants message "tokens" for a key (one key per channel).
type Limiter struct {
	pool  *pgxpool.Pool
	key   string
	limit int
}

// New creates a limiter allowing `perSecond` tokens per second for key.
func New(pool *pgxpool.Pool, key string, perSecond int) *Limiter {
	return &Limiter{pool: pool, key: key, limit: perSecond}
}

// Limit returns the configured tokens per second.
func (l *Limiter) Limit() int { return l.limit }

// Acquire blocks until at least one token is available and returns how many
// tokens (1..n) were granted. Callers loop until they acquired everything.
func (l *Limiter) Acquire(ctx context.Context, n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > l.limit {
		n = l.limit
	}
	ctx, span := tracing.StartChild(ctx, "ratelimit.acquire")
	span.SetAttributes(attribute.String("ratelimit.key", l.key), attribute.Int("ratelimit.requested", n))
	defer span.End()
	waits := 0
	for {
		var granted int
		if err := l.pool.QueryRow(ctx, AcquireTokensQuery, l.key, n, l.limit).Scan(&granted); err != nil {
			return 0, fmt.Errorf("acquire rate tokens: %w", err)
		}
		if granted > 0 {
			span.SetAttributes(attribute.Int("ratelimit.granted", granted), attribute.Int("ratelimit.waits", waits))
			return granted, nil
		}
		waits++
		// Window is exhausted: wait until the next second starts.
		wait := time.Until(time.Now().Truncate(time.Second).Add(time.Second)) + 5*time.Millisecond
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(wait):
		}
	}
}
