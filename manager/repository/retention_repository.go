package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/manager/queries"
)

// RetentionRepository deletes data older than the retention period.
type RetentionRepository struct {
	pool *pgxpool.Pool
}

// NewRetentionRepository creates the repository.
func NewRetentionRepository(pool *pgxpool.Pool) *RetentionRepository {
	return &RetentionRepository{pool: pool}
}

// DeleteDeliveries deletes one batch of old delivery rows of a channel.
func (r *RetentionRepository) DeleteDeliveries(ctx context.Context, channel string, days, limit int) (int64, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return 0, err
	}
	tag, err := r.pool.Exec(ctx, queries.DeleteOldDeliveries(table), days, limit)
	return tag.RowsAffected(), err
}

// DeleteNotifications deletes one batch of old notifications of a channel.
func (r *RetentionRepository) DeleteNotifications(ctx context.Context, channel string, days, limit int) (int64, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return 0, err
	}
	tag, err := r.pool.Exec(ctx, queries.DeleteOldNotifications(table), days, limit, channel)
	return tag.RowsAffected(), err
}

// DeleteLogs deletes one batch of old audit log rows.
func (r *RetentionRepository) DeleteLogs(ctx context.Context, days, limit int) (int64, error) {
	tag, err := r.pool.Exec(ctx, queries.DeleteOldLogs, days, limit)
	return tag.RowsAffected(), err
}
