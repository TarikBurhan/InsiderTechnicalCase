package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/customizer/queries"
	"notifier/pkg/correlation"
)

// LogRepository writes notification_logs entries.
type LogRepository struct {
	pool    *pgxpool.Pool
	service string
}

// NewLogRepository creates the repository.
func NewLogRepository(pool *pgxpool.Pool, service string) *LogRepository {
	return &LogRepository{pool: pool, service: service}
}

// Write inserts a log entry; the correlation id is taken from ctx.
func (r *LogRepository) Write(ctx context.Context, notificationID, channel, level, event, message string, details any) error {
	var raw []byte
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return err
		}
		raw = b
	}
	_, err := r.pool.Exec(ctx, queries.InsertLog, notificationID, channel, r.service, level, event, message,
		correlation.FromContext(ctx), raw)
	return err
}
