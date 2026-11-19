package queries

import (
	"fmt"

	"notifier/panel/dto"
)

// DeliveryTable returns the whitelisted delivery table for a channel.
func DeliveryTable(channel string) (string, error) {
	switch channel {
	case dto.ChannelSMS:
		return "sms_deliveries", nil
	case dto.ChannelEmail:
		return "email_deliveries", nil
	case dto.ChannelPush:
		return "push_deliveries", nil
	}
	return "", fmt.Errorf("unknown channel %q", channel)
}

// DeliveryStats aggregates delivery statuses of one notification.
func DeliveryStats(table string) string {
	return fmt.Sprintf(`SELECT status, count(*) FROM %s WHERE notification_id = $1::uuid GROUP BY status`, table)
}

// CancelDeliveries cancels deliveries that were not sent yet.
func CancelDeliveries(table string) string {
	return fmt.Sprintf(`
UPDATE %s SET status = 'CANCELLED', locked_until = NULL, next_retry_at = NULL, updated_at = now()
WHERE notification_id = $1::uuid AND status IN ('SENDING', 'RETRY')`, table)
}

// ListDeliveries pages through deliveries of a notification, optionally by status.
func ListDeliveries(table string) string {
	return fmt.Sprintf(`
SELECT id, notification_id::text, contact_id, recipient, batch_no, status, retry_count, next_retry_at,
       provider_message_id, last_error, sent_at, created_at, updated_at
FROM %s
WHERE notification_id = $1::uuid AND ($2 = '' OR status = $2)
ORDER BY id
LIMIT $3 OFFSET $4`, table)
}

// CountDeliveries counts deliveries of a notification, optionally by status.
func CountDeliveries(table string) string {
	return fmt.Sprintf(`SELECT count(*) FROM %s WHERE notification_id = $1::uuid AND ($2 = '' OR status = $2)`, table)
}
