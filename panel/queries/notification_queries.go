// Package queries contains every SQL statement used by the Panel. No ORM is
// used; repositories execute these strings with pgx.
package queries

import (
	"fmt"
	"strings"

	"notifier/panel/dto"
)

// NotificationColumns is the canonical select list for notifications.
const NotificationColumns = `id::text, batch_id::text, idempotency_key, correlation_id, channel, priority, status,
	audience_type, audience_group_id, audience_definition, audience_sql, recipients, content_id, content,
	customized_content, scheduled_at, total_recipients, error_message, processing_started_at,
	sending_started_at, completed_at, cancelled_at, created_at, updated_at`

// InsertNotification inserts a NEW notification. When an idempotency key is
// reused nothing is inserted and no row is returned.
const InsertNotification = `
INSERT INTO notifications (
	batch_id, idempotency_key, correlation_id, channel, priority, audience_type, audience_group_id,
	audience_definition, audience_sql, recipients, content_id, content, customized_content, scheduled_at
) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING ` + NotificationColumns

// GetNotificationByID loads a notification.
const GetNotificationByID = `SELECT ` + NotificationColumns + ` FROM notifications WHERE id = $1::uuid`

// GetNotificationByIdempotencyKey loads a notification by idempotency key.
const GetNotificationByIdempotencyKey = `SELECT ` + NotificationColumns + ` FROM notifications WHERE idempotency_key = $1`

// ListNotificationsByBatch loads every notification of a batch.
const ListNotificationsByBatch = `SELECT ` + NotificationColumns + ` FROM notifications WHERE batch_id = $1::uuid ORDER BY created_at, id`

// CancelNotification cancels a notification that is not finished yet.
const CancelNotification = `
UPDATE notifications
SET status = 'CANCELLED', cancelled_at = now(), updated_at = now()
WHERE id = $1::uuid AND status IN ('NEW', 'PROCESSING', 'SENDING')
RETURNING ` + NotificationColumns

// BuildListNotifications builds the filtered/paginated list query and the
// matching count query with positional parameters.
func BuildListNotifications(f dto.NotificationFilter) (listSQL, countSQL string, args []any) {
	var where []string
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Channel != "" {
		add("channel = $%d", f.Channel)
	}
	if f.Priority != "" {
		add("priority = $%d", f.Priority)
	}
	if f.BatchID != "" {
		add("batch_id = $%d::uuid", f.BatchID)
	}
	if f.From != nil {
		add("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("created_at <= $%d", *f.To)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	countSQL = "SELECT count(*) FROM notifications" + clause
	listSQL = fmt.Sprintf("SELECT %s FROM notifications%s ORDER BY created_at DESC, id LIMIT $%d OFFSET $%d",
		NotificationColumns, clause, len(args)+1, len(args)+2)
	return listSQL, countSQL, args
}
