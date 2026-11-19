package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/pushsender/queries"
)

// NotificationRepository updates notification status.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates the repository.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// CompleteIfDone marks the notification COMPLETE when nothing is pending.
func (r *NotificationRepository) CompleteIfDone(ctx context.Context, notificationID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, queries.CompleteIfDone, notificationID)
	return tag.RowsAffected() == 1, err
}
