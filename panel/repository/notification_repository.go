package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// NotificationRepository persists notification requests.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates the repository.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (dto.Notification, error) {
	var n dto.Notification
	err := row.Scan(&n.ID, &n.BatchID, &n.IdempotencyKey, &n.CorrelationID, &n.Channel, &n.Priority, &n.Status,
		&n.AudienceType, &n.AudienceGroupID, &n.AudienceDefinition, &n.AudienceSQL, &n.Recipients, &n.ContentID,
		&n.Content, &n.CustomizedContent, &n.ScheduledAt, &n.TotalRecipients, &n.ErrorMessage,
		&n.ProcessingStarted, &n.SendingStarted, &n.CompletedAt, &n.CancelledAt, &n.CreatedAt, &n.UpdatedAt, &n.TraceContext, &n.AudienceCount)
	return n, err
}

func insertArgs(n dto.NewNotification) []any {
	return []any{n.BatchID, n.IdempotencyKey, n.CorrelationID, n.Channel, n.Priority, n.AudienceType,
		n.AudienceGroupID, nullJSON(n.AudienceDefinition), n.AudienceSQL, nullJSON(n.Recipients), n.ContentID,
		[]byte(n.Content), n.CustomizedContent, n.ScheduledAt, n.TraceContext, n.Status, n.AudienceCount}
}

// Insert creates a notification. created is false when the idempotency key
// already existed; in that case the existing notification is returned.
func (r *NotificationRepository) Insert(ctx context.Context, n dto.NewNotification) (dto.Notification, bool, error) {
	out, err := scanNotification(r.pool.QueryRow(ctx, queries.InsertNotification, insertArgs(n)...))
	if err == nil {
		return out, true, nil
	}
	if err == pgx.ErrNoRows && n.IdempotencyKey != nil {
		existing, getErr := r.GetByIdempotencyKey(ctx, *n.IdempotencyKey)
		return existing, false, getErr
	}
	return dto.Notification{}, false, mapError(err)
}

// InsertBatch inserts many notifications in one transaction using a pgx
// batch (one round trip). Duplicates by idempotency key return the
// existing row with created=false.
func (r *NotificationRepository) InsertBatch(ctx context.Context, items []dto.NewNotification) ([]dto.Notification, []bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, n := range items {
		batch.Queue(queries.InsertNotification, insertArgs(n)...)
	}
	results := tx.SendBatch(ctx, batch)
	out := make([]dto.Notification, len(items))
	created := make([]bool, len(items))
	var dupIdx []int
	for i := range items {
		n, err := scanNotification(results.QueryRow())
		switch {
		case err == nil:
			out[i], created[i] = n, true
		case err == pgx.ErrNoRows && items[i].IdempotencyKey != nil:
			dupIdx = append(dupIdx, i)
		default:
			_ = results.Close()
			return nil, nil, fmt.Errorf("insert notification %d: %w", i, mapError(err))
		}
	}
	if err := results.Close(); err != nil {
		return nil, nil, err
	}
	for _, i := range dupIdx {
		n, err := scanNotification(tx.QueryRow(ctx, queries.GetNotificationByIdempotencyKey, *items[i].IdempotencyKey))
		if err != nil {
			return nil, nil, mapError(err)
		}
		out[i] = n
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return out, created, nil
}

// GetByID loads a notification.
func (r *NotificationRepository) GetByID(ctx context.Context, id string) (dto.Notification, error) {
	n, err := scanNotification(r.pool.QueryRow(ctx, queries.GetNotificationByID, id))
	return n, mapError(err)
}

// GetByIdempotencyKey loads a notification by its idempotency key.
func (r *NotificationRepository) GetByIdempotencyKey(ctx context.Context, key string) (dto.Notification, error) {
	n, err := scanNotification(r.pool.QueryRow(ctx, queries.GetNotificationByIdempotencyKey, key))
	return n, mapError(err)
}

// ListByBatch loads every notification of a batch.
func (r *NotificationRepository) ListByBatch(ctx context.Context, batchID string) ([]dto.Notification, error) {
	rows, err := r.pool.Query(ctx, queries.ListNotificationsByBatch, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// List returns a filtered page of notifications and the total count.
func (r *NotificationRepository) List(ctx context.Context, f dto.NotificationFilter) ([]dto.Notification, int64, error) {
	listSQL, countSQL, args := queries.BuildListNotifications(f)
	var total int64
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, listSQL, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]dto.Notification, 0, f.Limit)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	return out, total, rows.Err()
}

// Cancel cancels the notification and every pending delivery atomically.
func (r *NotificationRepository) Cancel(ctx context.Context, id string) (dto.Notification, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return dto.Notification{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	n, err := scanNotification(tx.QueryRow(ctx, queries.CancelNotification, id))
	if err != nil {
		return dto.Notification{}, mapError(err)
	}
	table, err := queries.DeliveryTable(n.Channel)
	if err != nil {
		return dto.Notification{}, err
	}
	if _, err := tx.Exec(ctx, queries.CancelDeliveries(table), id); err != nil {
		return dto.Notification{}, err
	}
	return n, tx.Commit(ctx)
}
