package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/manager/dto"
	"notifier/manager/queries"
)

// AudienceQueryError wraps an error raised by the database while executing
// the audience SQL itself (syntax, permission...). These are permanent.
type AudienceQueryError struct{ Err error }

func (e *AudienceQueryError) Error() string { return "audience query failed: " + e.Err.Error() }
func (e *AudienceQueryError) Unwrap() error { return e.Err }

// AudienceRepository resolves audiences with the read-only audience role.
type AudienceRepository struct {
	readerPool *pgxpool.Pool
	timeout    time.Duration
}

// NewAudienceRepository creates the repository.
func NewAudienceRepository(readerPool *pgxpool.Pool, timeout time.Duration) *AudienceRepository {
	return &AudienceRepository{readerPool: readerPool, timeout: timeout}
}

// Stream executes the audience SQL inside a READ ONLY transaction and calls
// fn with consecutive batches of at most batchSize contacts. Streaming keeps
// memory flat even for audiences of millions of contacts.
func (r *AudienceRepository) Stream(ctx context.Context, audienceSQL, channel string, batchSize int,
	fn func([]dto.AudienceContact) error) error {

	tx, err := r.readerPool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, queries.SetStatementTimeout(r.timeout.Milliseconds())); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, queries.StreamAudience(audienceSQL, dto.AddressColumn(channel), dto.AddressKeyExpr(channel)))
	if err != nil {
		return wrapQueryError(err)
	}
	defer rows.Close()

	batch := make([]dto.AudienceContact, 0, batchSize)
	for rows.Next() {
		var id int64
		var address string
		if err := rows.Scan(&id, &address); err != nil {
			return fmt.Errorf("scan audience row: %w", err)
		}
		batch = append(batch, dto.AudienceContact{ContactID: &id, Address: address})
		if len(batch) == batchSize {
			if err := fn(batch); err != nil {
				return err
			}
			batch = make([]dto.AudienceContact, 0, batchSize)
		}
	}
	if err := rows.Err(); err != nil {
		return wrapQueryError(err)
	}
	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}

func wrapQueryError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code != "57014" { // 57014 = query_canceled (timeout) -> retryable
		return &AudienceQueryError{Err: err}
	}
	return err
}
