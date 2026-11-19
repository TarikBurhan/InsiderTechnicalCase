// Package repository executes the Manager's SQL with pgx.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/manager/dto"
	"notifier/manager/queries"
)

// NotificationRepository manages notification state transitions.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates the repository.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

func collectNotifications(rows pgx.Rows) ([]dto.Notification, error) {
	defer rows.Close()
	var out []dto.Notification
	for rows.Next() {
		var n dto.Notification
		if err := rows.Scan(&n.ID, &n.CorrelationID, &n.Channel, &n.Priority, &n.Status, &n.AudienceType,
			&n.AudienceSQL, &n.Recipients, &n.Content, &n.CustomizedContent, &n.ProcessingAttempts); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ClaimNew claims up to limit due NEW notifications.
func (r *NotificationRepository) ClaimNew(ctx context.Context, limit int) ([]dto.Notification, error) {
	rows, err := r.pool.Query(ctx, queries.ClaimNewNotifications, limit)
	if err != nil {
		return nil, err
	}
	return collectNotifications(rows)
}

// ClaimStale claims PROCESSING notifications without a recent heartbeat.
func (r *NotificationRepository) ClaimStale(ctx context.Context, limit int, staleAfter time.Duration) ([]dto.Notification, error) {
	rows, err := r.pool.Query(ctx, queries.ClaimStaleProcessing, limit, int(staleAfter.Seconds()))
	if err != nil {
		return nil, err
	}
	return collectNotifications(rows)
}

// Load loads notifications by id.
func (r *NotificationRepository) Load(ctx context.Context, ids []string) (map[string]dto.Notification, error) {
	rows, err := r.pool.Query(ctx, queries.LoadNotifications, ids)
	if err != nil {
		return nil, err
	}
	list, err := collectNotifications(rows)
	if err != nil {
		return nil, err
	}
	out := make(map[string]dto.Notification, len(list))
	for _, n := range list {
		out[n.ID] = n
	}
	return out, nil
}

// Heartbeat refreshes the heartbeat of a PROCESSING notification.
func (r *NotificationRepository) Heartbeat(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, queries.Heartbeat, id)
	return err
}

// Status returns the current status.
func (r *NotificationRepository) Status(ctx context.Context, id string) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, queries.GetStatus, id).Scan(&status)
	return status, err
}

// MarkSending moves PROCESSING -> SENDING.
func (r *NotificationRepository) MarkSending(ctx context.Context, id string, total int) (bool, error) {
	tag, err := r.pool.Exec(ctx, queries.MarkSending, id, total)
	return tag.RowsAffected() == 1, err
}

// MarkError moves a notification to ERROR.
func (r *NotificationRepository) MarkError(ctx context.Context, id, message string) error {
	_, err := r.pool.Exec(ctx, queries.MarkError, id, message)
	return err
}

// CompleteIfDone completes a SENDING notification without pending deliveries.
func (r *NotificationRepository) CompleteIfDone(ctx context.Context, channel, id string) (bool, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return false, err
	}
	tag, err := r.pool.Exec(ctx, queries.CompleteIfDone(table), id)
	return tag.RowsAffected() == 1, err
}

// CompleteFinished completes every finished SENDING notification of a channel.
func (r *NotificationRepository) CompleteFinished(ctx context.Context, channel string) ([]dto.CompletedNotification, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, queries.CompleteFinished(table), channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.CompletedNotification
	for rows.Next() {
		var c dto.CompletedNotification
		if err := rows.Scan(&c.ID, &c.CorrelationID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
