package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/pkg/correlation"
	"notifier/pkg/tracing"
	"notifier/smssender/config"
	"notifier/smssender/dto"
	"notifier/smssender/queries"
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

// Write inserts a single log entry.
func (r *LogRepository) Write(ctx context.Context, e dto.LogEntry) error {
	_, err := r.pool.Exec(ctx, queries.InsertLog, e.NotificationID, e.DeliveryID, config.Channel, r.service,
		e.Level, e.Event, e.Message, correlation.FromContext(ctx), e.Details, tracing.TraceID(ctx))
	return err
}

// WriteMany bulk inserts log entries with COPY (one round trip per batch).
func (r *LogRepository) WriteMany(ctx context.Context, entries []dto.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	corr := correlation.FromContext(ctx)
	var traceID any
	if id := tracing.TraceID(ctx); id != "" {
		traceID = id
	}
	rows := make([][]any, len(entries))
	for i, e := range entries {
		var nid pgtype.UUID
		if err := nid.Scan(e.NotificationID); err != nil {
			return err
		}
		var details any
		if len(e.Details) > 0 {
			details = string(e.Details)
		}
		rows[i] = []any{nid, e.DeliveryID, config.Channel, r.service, e.Level, e.Event, e.Message, corr, details, traceID}
	}
	_, err := r.pool.CopyFrom(ctx, pgx.Identifier{queries.LogTable}, queries.LogColumns, pgx.CopyFromRows(rows))
	return err
}
