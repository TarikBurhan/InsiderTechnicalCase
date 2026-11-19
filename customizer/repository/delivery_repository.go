package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/customizer/dto"
	"notifier/customizer/queries"
)

// DeliveryRepository updates delivery rows that fail personalisation.
type DeliveryRepository struct {
	pool *pgxpool.Pool
}

// NewDeliveryRepository creates the repository.
func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{pool: pool}
}

// MarkFailed sets the given deliveries to ERROR with a reason.
func (r *DeliveryRepository) MarkFailed(ctx context.Context, channel string, failed []dto.FailedRecipient) error {
	if len(failed) == 0 {
		return nil
	}
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return err
	}
	ids := make([]int64, len(failed))
	reasons := make([]string, len(failed))
	for i, f := range failed {
		ids[i], reasons[i] = f.DeliveryID, f.Reason
	}
	_, err = r.pool.Exec(ctx, queries.MarkDeliveriesFailed(table), ids, reasons)
	return err
}
