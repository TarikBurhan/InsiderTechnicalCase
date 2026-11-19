package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/manager/dto"
	"notifier/manager/queries"
)

// DeliveryRepository writes the per-channel delivery tables.
type DeliveryRepository struct {
	pool *pgxpool.Pool
}

// NewDeliveryRepository creates the repository.
func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{pool: pool}
}

// InsertBatch stores a batch of audience members as SENDING deliveries and
// returns the rows that must be produced.
func (r *DeliveryRepository) InsertBatch(ctx context.Context, channel, notificationID string, batchNo int, contacts []dto.AudienceContact) ([]dto.DeliveryTarget, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return nil, err
	}
	contactIDs := make([]*int64, len(contacts))
	recipients := make([]string, len(contacts))
	for i, c := range contacts {
		contactIDs[i], recipients[i] = c.ContactID, c.Address
	}
	rows, err := r.pool.Query(ctx, queries.InsertDeliveries(table), notificationID, batchNo, contactIDs, recipients)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]dto.DeliveryTarget, 0, len(contacts))
	for rows.Next() {
		var t dto.DeliveryTarget
		if err := rows.Scan(&t.DeliveryID, &t.ContactID, &t.Recipient); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimDueRetries claims RETRY rows whose next_retry_at has passed.
func (r *DeliveryRepository) ClaimDueRetries(ctx context.Context, channel string, limit int) ([]dto.RetryDelivery, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, queries.ClaimDueRetries(table), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.RetryDelivery
	for rows.Next() {
		var d dto.RetryDelivery
		if err := rows.Scan(&d.DeliveryID, &d.NotificationID, &d.ContactID, &d.Recipient, &d.RetryCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Reschedule moves SENDING rows back to RETRY after delay.
func (r *DeliveryRepository) Reschedule(ctx context.Context, channel string, ids []int64, delay time.Duration) error {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, queries.RescheduleRetries(table), ids, int(delay.Seconds()))
	return err
}

// Cancel cancels the given rows.
func (r *DeliveryRepository) Cancel(ctx context.Context, channel string, ids []int64) error {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, queries.CancelDeliveries(table), ids)
	return err
}

// SweepStale moves rows stuck in SENDING back to RETRY.
func (r *DeliveryRepository) SweepStale(ctx context.Context, channel string, staleAfter time.Duration, limit int) ([]dto.SweptDelivery, error) {
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, queries.SweepStaleSending(table), int(staleAfter.Seconds()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.SweptDelivery
	for rows.Next() {
		var s dto.SweptDelivery
		if err := rows.Scan(&s.DeliveryID, &s.NotificationID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
