// Package repository executes the Push sender's SQL with pgx.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/pushsender/dto"
	"notifier/pushsender/queries"
)

// DeliveryRepository updates push_deliveries.
type DeliveryRepository struct {
	pool *pgxpool.Pool
}

// NewDeliveryRepository creates the repository.
func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{pool: pool}
}

// Claim leases the given deliveries and returns the ids this consumer owns.
func (r *DeliveryRepository) Claim(ctx context.Context, ids []int64, lease time.Duration) (map[int64]bool, error) {
	rows, err := r.pool.Query(ctx, queries.ClaimDeliveries, ids, int(lease.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]bool, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// MarkSent records successful sends.
func (r *DeliveryRepository) MarkSent(ctx context.Context, results []dto.SendResult) error {
	if len(results) == 0 {
		return nil
	}
	ids := make([]int64, len(results))
	msgIDs := make([]string, len(results))
	for i, res := range results {
		ids[i], msgIDs[i] = res.DeliveryID, res.ProviderMessageID
	}
	_, err := r.pool.Exec(ctx, queries.MarkSent, ids, msgIDs)
	return err
}

// MarkFailed records failed sends and returns their new status.
func (r *DeliveryRepository) MarkFailed(ctx context.Context, results []dto.SendResult, maxRetries int, retryDelay time.Duration) ([]dto.FailedDelivery, error) {
	if len(results) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(results))
	errs := make([]string, len(results))
	permanent := make([]bool, len(results))
	for i, res := range results {
		ids[i], permanent[i] = res.DeliveryID, res.Permanent
		if res.Err != nil {
			errs[i] = truncate(res.Err.Error(), 1000)
		}
	}
	rows, err := r.pool.Query(ctx, queries.MarkFailed, ids, maxRetries, int(retryDelay.Seconds()), errs, permanent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.FailedDelivery
	for rows.Next() {
		var f dto.FailedDelivery
		if err := rows.Scan(&f.DeliveryID, &f.Status, &f.RetryCount, &f.Error); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
