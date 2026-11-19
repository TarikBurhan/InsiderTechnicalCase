package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// LogRepository writes and reads notification_logs.
type LogRepository struct {
	pool    *pgxpool.Pool
	service string
}

// NewLogRepository creates the repository.
func NewLogRepository(pool *pgxpool.Pool, service string) *LogRepository {
	return &LogRepository{pool: pool, service: service}
}

// Write inserts a log entry.
func (r *LogRepository) Write(ctx context.Context, notificationID *string, channel, level, event, message, correlationID string, details any) error {
	var raw []byte
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return err
		}
		raw = b
	}
	_, err := r.pool.Exec(ctx, queries.InsertLog, notificationID, channel, r.service, level, event, message, correlationID, raw)
	return err
}

// List pages through the logs of a notification.
func (r *LogRepository) List(ctx context.Context, notificationID string, limit, offset int) ([]dto.NotificationLog, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, queries.CountLogs, notificationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, queries.ListLogs, notificationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]dto.NotificationLog, 0, limit)
	for rows.Next() {
		var l dto.NotificationLog
		if err := rows.Scan(&l.ID, &l.NotificationID, &l.DeliveryID, &l.Channel, &l.Service, &l.Level, &l.Event,
			&l.Message, &l.CorrelationID, &l.Details, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}
