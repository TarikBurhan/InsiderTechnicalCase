package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// DeliveryRepository reads the per-channel delivery tables.
type DeliveryRepository struct {
	pool *pgxpool.Pool
}

// NewDeliveryRepository creates the repository.
func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{pool: pool}
}

// Stats aggregates delivery statuses of a notification.
func (r *DeliveryRepository) Stats(ctx context.Context, channel, notificationID string) (dto.DeliveryStats, error) {
	var s dto.DeliveryStats
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return s, err
	}
	rows, err := r.pool.Query(ctx, queries.DeliveryStats(table), notificationID)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return s, err
		}
		s.Total += count
		switch status {
		case dto.DeliverySending:
			s.Sending = count
		case dto.DeliverySent:
			s.Sent = count
		case dto.DeliveryRetry:
			s.Retry = count
		case dto.DeliveryError:
			s.Error = count
		case dto.DeliveryCancelled:
			s.Cancelled = count
		}
	}
	return s, rows.Err()
}

// List pages through deliveries of a notification.
func (r *DeliveryRepository) List(ctx context.Context, channel, notificationID, status string, limit, offset int) ([]dto.Delivery, int64, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.pool.QueryRow(ctx, queries.CountDeliveries(table), notificationID, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, queries.ListDeliveries(table), notificationID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]dto.Delivery, 0, limit)
	for rows.Next() {
		var d dto.Delivery
		if err := rows.Scan(&d.ID, &d.NotificationID, &d.ContactID, &d.Recipient, &d.BatchNo, &d.Status, &d.RetryCount,
			&d.NextRetryAt, &d.ProviderMessageID, &d.LastError, &d.SentAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}
