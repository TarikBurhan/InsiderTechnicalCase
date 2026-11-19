package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/panel/dto"
	"notifier/panel/queries"
)

// ContentRepository stores reusable channel contents/templates.
type ContentRepository struct {
	pool *pgxpool.Pool
}

// NewContentRepository creates the repository.
func NewContentRepository(pool *pgxpool.Pool) *ContentRepository {
	return &ContentRepository{pool: pool}
}

func scanContent(row rowScanner, channel string) (dto.Content, error) {
	c := dto.Content{Channel: channel}
	err := row.Scan(&c.ID, &c.Name, &c.Subject, &c.Title, &c.Body, &c.IsHTML, &c.ImageURL, &c.Data, &c.Variables,
		&c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// Insert saves a content in the table of its channel.
func (r *ContentRepository) Insert(ctx context.Context, c dto.Content) (dto.Content, error) {
	var (
		sql  string
		args []any
	)
	switch c.Channel {
	case dto.ChannelSMS:
		sql, args = queries.InsertSMSContent, []any{c.Name, c.Body, c.Variables}
	case dto.ChannelEmail:
		sql, args = queries.InsertEmailContent, []any{c.Name, c.Subject, c.Body, c.IsHTML, c.Variables}
	case dto.ChannelPush:
		sql, args = queries.InsertPushContent, []any{c.Name, c.Title, c.Body, c.ImageURL, nullJSON(c.Data), c.Variables}
	default:
		return dto.Content{}, fmt.Errorf("unknown channel %q", c.Channel)
	}
	out, err := scanContent(r.pool.QueryRow(ctx, sql, args...), c.Channel)
	return out, mapError(err)
}

// Get loads a content by channel and id.
func (r *ContentRepository) Get(ctx context.Context, channel string, id int64) (dto.Content, error) {
	sql, err := queries.GetContent(channel)
	if err != nil {
		return dto.Content{}, err
	}
	c, err := scanContent(r.pool.QueryRow(ctx, sql, id), channel)
	return c, mapError(err)
}

// List pages through contents of a channel.
func (r *ContentRepository) List(ctx context.Context, channel string, limit, offset int) ([]dto.Content, int64, error) {
	countSQL, err := queries.CountContents(channel)
	if err != nil {
		return nil, 0, err
	}
	listSQL, _ := queries.ListContents(channel)
	var total int64
	if err := r.pool.QueryRow(ctx, countSQL).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, listSQL, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]dto.Content, 0, limit)
	for rows.Next() {
		c, err := scanContent(rows, channel)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Delete removes a content.
func (r *ContentRepository) Delete(ctx context.Context, channel string, id int64) error {
	sql, err := queries.DeleteContent(channel)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
