package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// AudienceRepository validates audiences and stores audience groups.
//
// readerPool connects as the `audience_reader` role which can only SELECT
// the contact tables, so user supplied SQL can never read or modify other
// data even if it slipped through static validation.
type AudienceRepository struct {
	pool       *pgxpool.Pool
	readerPool *pgxpool.Pool
}

// NewAudienceRepository creates the repository.
func NewAudienceRepository(pool, readerPool *pgxpool.Pool) *AudienceRepository {
	return &AudienceRepository{pool: pool, readerPool: readerPool}
}

// CountAudience executes the audience SQL as the read-only role inside a
// READ ONLY transaction with a statement timeout and returns how many
// existing master_contact rows it selects.
func (r *AudienceRepository) CountAudience(ctx context.Context, audienceSQL string, timeout time.Duration) (int64, error) {
	tx, err := r.readerPool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ms := timeout.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	if _, err := tx.Exec(ctx, queries.SetStatementTimeout(ms)); err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, queries.CountAudience(audienceSQL)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// IsTimeout reports whether err is a statement timeout / deadline.
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "57014" // query_canceled
}

// TableSchema loads the column list of the given tables.
func (r *AudienceRepository) TableSchema(ctx context.Context, tables []string) (dto.TableSchema, error) {
	rows, err := r.pool.Query(ctx, queries.TableColumns, tables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	schema := dto.TableSchema{}
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&table, &column, &dataType); err != nil {
			return nil, err
		}
		if schema[table] == nil {
			schema[table] = map[string]string{}
		}
		schema[table][column] = dataType
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _, ok := schema["master_contact"]; !ok {
		return nil, fmt.Errorf("master_contact table not found")
	}
	return schema, nil
}

func scanAudienceGroup(row rowScanner) (dto.AudienceGroup, error) {
	var g dto.AudienceGroup
	err := row.Scan(&g.ID, &g.Name, &g.Description, &g.SourceType, &g.Definition, &g.SQLQuery, &g.ContactCount,
		&g.ValidatedAt, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

// InsertGroup saves a validated audience group.
func (r *AudienceRepository) InsertGroup(ctx context.Context, g dto.AudienceGroup) (dto.AudienceGroup, error) {
	out, err := scanAudienceGroup(r.pool.QueryRow(ctx, queries.InsertAudienceGroup,
		g.Name, g.Description, g.SourceType, nullJSON(g.Definition), g.SQLQuery, g.ContactCount))
	return out, mapError(err)
}

// GetGroup loads an audience group.
func (r *AudienceRepository) GetGroup(ctx context.Context, id int64) (dto.AudienceGroup, error) {
	g, err := scanAudienceGroup(r.pool.QueryRow(ctx, queries.GetAudienceGroup, id))
	return g, mapError(err)
}

// ListGroups pages through audience groups.
func (r *AudienceRepository) ListGroups(ctx context.Context, limit, offset int) ([]dto.AudienceGroup, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, queries.CountAudienceGroups).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, queries.ListAudienceGroups, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]dto.AudienceGroup, 0, limit)
	for rows.Next() {
		g, err := scanAudienceGroup(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

// DeleteGroup deletes an audience group.
func (r *AudienceRepository) DeleteGroup(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, queries.DeleteAudienceGroup, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
