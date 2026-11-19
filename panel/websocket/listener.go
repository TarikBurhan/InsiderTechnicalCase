package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
)

// StatusChannel is the PostgreSQL NOTIFY channel fed by the
// trg_notifications_status trigger.
const StatusChannel = "notification_status"

// Listener LISTENs on PostgreSQL and forwards events to the hub.
type Listener struct {
	pool *pgxpool.Pool
	hub  *Hub
	log  *slog.Logger
}

// NewListener creates a listener.
func NewListener(pool *pgxpool.Pool, hub *Hub, log *slog.Logger) *Listener {
	return &Listener{pool: pool, hub: hub, log: log}
}

// Run listens until ctx is cancelled, reconnecting on errors.
func (l *Listener) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := l.listen(ctx); err != nil && ctx.Err() == nil {
			l.log.Warn("status listener stopped, retrying", slog.Any("error", err))
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (l *Listener) listen(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+StatusChannel); err != nil {
		return err
	}
	l.log.Info("listening for notification status changes")
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			// Never hand a LISTENing connection back to the pool.
			_ = conn.Conn().Close(context.Background())
			return err
		}
		var event dto.StatusEvent
		if err := json.Unmarshal([]byte(n.Payload), &event); err != nil {
			l.log.Warn("invalid status payload", slog.Any("error", err))
			continue
		}
		l.hub.Broadcast(event)
	}
}
