// Package repository executes the Customizer's SQL with pgx.
package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/customizer/dto"
	"notifier/customizer/queries"
)

// ContactRepository loads contact data used for variable substitution.
type ContactRepository struct {
	readerPool *pgxpool.Pool
}

// NewContactRepository creates the repository.
func NewContactRepository(readerPool *pgxpool.Pool) *ContactRepository {
	return &ContactRepository{readerPool: readerPool}
}

// Load returns contact id -> column variables for the given ids.
func (r *ContactRepository) Load(ctx context.Context, ids []int64) (map[int64]dto.ContactVariables, error) {
	out := make(map[int64]dto.ContactVariables, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.readerPool.Query(ctx, queries.LoadContacts, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		vars := make(dto.ContactVariables, len(fields))
		var id int64
		for i, f := range fields {
			if f.Name == "id" {
				if v, ok := values[i].(int64); ok {
					id = v
				}
			}
			vars[f.Name] = formatValue(values[i], f.DataTypeOID)
		}
		out[id] = vars
	}
	return out, rows.Err()
}

func formatValue(v any, oid uint32) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case time.Time:
		if oid == pgtype.DateOID {
			return val.Format("2006-01-02")
		}
		return val.Format(time.RFC3339)
	case int64:
		return strconv.FormatInt(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprint(val)
	}
}
