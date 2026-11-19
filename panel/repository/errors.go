// Package repository executes the Panel's SQL (from package queries) with
// pgx and maps rows to DTOs.
package repository

import (
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned on unique constraint violations.
var ErrDuplicate = errors.New("duplicate")

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}

// nullJSON converts an empty raw message to a SQL NULL.
func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}
