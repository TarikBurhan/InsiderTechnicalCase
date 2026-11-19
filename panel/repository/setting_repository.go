package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// SettingRepository reads and writes account_settings.
type SettingRepository struct {
	pool *pgxpool.Pool
}

// NewSettingRepository creates the repository.
func NewSettingRepository(pool *pgxpool.Pool) *SettingRepository {
	return &SettingRepository{pool: pool}
}

func scanSetting(row rowScanner) (dto.Setting, error) {
	var s dto.Setting
	err := row.Scan(&s.Key, &s.Value, &s.Description, &s.UpdatedAt)
	return s, err
}

// List returns every setting.
func (r *SettingRepository) List(ctx context.Context) ([]dto.Setting, error) {
	rows, err := r.pool.Query(ctx, queries.ListSettings)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dto.Setting
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Get returns one setting.
func (r *SettingRepository) Get(ctx context.Context, key string) (dto.Setting, error) {
	s, err := scanSetting(r.pool.QueryRow(ctx, queries.GetSetting, key))
	return s, mapError(err)
}

// Update changes a setting value.
func (r *SettingRepository) Update(ctx context.Context, key, value string) (dto.Setting, error) {
	s, err := scanSetting(r.pool.QueryRow(ctx, queries.UpdateSetting, key, value))
	return s, mapError(err)
}

// TimezoneExists reports whether PostgreSQL knows the time zone.
func (r *SettingRepository) TimezoneExists(ctx context.Context, name string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, queries.TimezoneExists, name).Scan(&ok)
	return ok, err
}

// DatabaseTime returns the time zones and clock of the database.
func (r *SettingRepository) DatabaseTime(ctx context.Context) (dto.DatabaseTime, error) {
	var t dto.DatabaseTime
	err := r.pool.QueryRow(ctx, queries.DatabaseTime).Scan(&t.SessionTimezone, &t.DatabaseTimezone, &t.Now)
	return t, err
}
