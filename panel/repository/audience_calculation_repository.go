package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// AudienceCalculationRepository manages AUDIENCE_CALCULATING notifications.
type AudienceCalculationRepository struct {
	pool *pgxpool.Pool
}

// NewAudienceCalculationRepository creates the repository.
func NewAudienceCalculationRepository(pool *pgxpool.Pool) *AudienceCalculationRepository {
	return &AudienceCalculationRepository{pool: pool}
}

// Claim takes up to limit calculations whose lease expired.
func (r *AudienceCalculationRepository) Claim(ctx context.Context, limit int, lease time.Duration) ([]dto.AudienceCalculation, error) {
	rows, err := r.pool.Query(ctx, queries.ClaimAudienceCalculations, limit, int(lease.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.AudienceCalculation
	for rows.Next() {
		var c dto.AudienceCalculation
		if err := rows.Scan(&c.NotificationID, &c.Channel, &c.CorrelationID, &c.AudienceSQL, &c.TraceContext); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Heartbeat extends the lease of a calculation.
func (r *AudienceCalculationRepository) Heartbeat(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, queries.HeartbeatAudienceCalculation, id)
	return err
}

// Complete stores the count and moves the notification to NEW.
func (r *AudienceCalculationRepository) Complete(ctx context.Context, id string, count int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, queries.CompleteAudienceCalculation, id, count)
	return tag.RowsAffected() == 1, err
}

// Fail moves the notification to ERROR.
func (r *AudienceCalculationRepository) Fail(ctx context.Context, id, message string) (bool, error) {
	tag, err := r.pool.Exec(ctx, queries.FailAudienceCalculation, id, message)
	return tag.RowsAffected() == 1, err
}
