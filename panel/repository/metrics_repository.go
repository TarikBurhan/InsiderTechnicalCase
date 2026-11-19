package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// MetricsRepository runs the aggregate queries behind /api/v1/metrics.
type MetricsRepository struct {
	pool *pgxpool.Pool
}

// NewMetricsRepository creates the repository.
func NewMetricsRepository(pool *pgxpool.Pool) *MetricsRepository {
	return &MetricsRepository{pool: pool}
}

// StatusCounts counts notifications per status in the last 24h.
func (r *MetricsRepository) StatusCounts(ctx context.Context) ([]dto.StatusCount, error) {
	rows, err := r.pool.Query(ctx, queries.NotificationStatusCounts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.StatusCount
	for rows.Next() {
		var sc dto.StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// ChannelStats aggregates delivery outcomes of a channel in a time window.
func (r *MetricsRepository) ChannelStats(ctx context.Context, channel string, windowSeconds int) (dto.ChannelDeliveryStat, error) {
	s := dto.ChannelDeliveryStat{Channel: channel}
	table, err := queries.DeliveryTable(channel)
	if err != nil {
		return s, err
	}
	err = r.pool.QueryRow(ctx, queries.ChannelDeliveryStats(table), windowSeconds).
		Scan(&s.Sent, &s.Failed, &s.Retrying, &s.InFlight, &s.AvgLatencyMs, &s.P95LatencyMs)
	return s, err
}
