package queries

import (
	"fmt"

	"notifier/manager/dto"
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

// InsertDeliveries writes one batch of audience members as SENDING rows.
// Rows that already exist (re-processing after a crash) are returned again
// only while still SENDING, so they get re-produced but never re-sent once
// they reached SENT/ERROR (senders also claim rows before sending).
func InsertDeliveries(table string) string {
	return fmt.Sprintf(`
INSERT INTO %[1]s (notification_id, contact_id, recipient, batch_no, status)
SELECT $1::uuid, u.contact_id, u.recipient, $2, 'SENDING'
FROM unnest($3::bigint[], $4::text[]) AS u(contact_id, recipient)
ON CONFLICT (notification_id, recipient) DO UPDATE SET updated_at = now()
WHERE %[1]s.status = 'SENDING'
RETURNING id, contact_id, recipient`, table)
}

// ClaimDueRetries moves due RETRY rows back to SENDING so they can be
// re-produced. SKIP LOCKED keeps concurrent Managers from double claiming.
func ClaimDueRetries(table string) string {
	return fmt.Sprintf(`
UPDATE %[1]s d
SET status = 'SENDING', next_retry_at = NULL, updated_at = now()
WHERE d.id IN (
	SELECT id FROM %[1]s
	WHERE status = 'RETRY' AND next_retry_at <= now()
	ORDER BY next_retry_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
)
RETURNING d.id, d.notification_id::text, d.contact_id, d.recipient, d.retry_count`, table)
}

// RescheduleRetries puts rows back to RETRY (e.g. when publishing failed).
func RescheduleRetries(table string) string {
	return fmt.Sprintf(`
UPDATE %s SET status = 'RETRY', next_retry_at = now() + $2::int * INTERVAL '1 second', updated_at = now()
WHERE id = ANY($1::bigint[]) AND status = 'SENDING'`, table)
}

// CancelDeliveries cancels the given rows.
func CancelDeliveries(table string) string {
	return fmt.Sprintf(`
UPDATE %s SET status = 'CANCELLED', locked_until = NULL, next_retry_at = NULL, updated_at = now()
WHERE id = ANY($1::bigint[]) AND status IN ('SENDING', 'RETRY')`, table)
}

// SweepStaleSending moves rows stuck in SENDING (message lost or consumer
// died after claiming) back to RETRY without consuming a retry attempt.
func SweepStaleSending(table string) string {
	return fmt.Sprintf(`
UPDATE %[1]s d
SET status = 'RETRY', next_retry_at = now(), updated_at = now()
WHERE d.id IN (
	SELECT id FROM %[1]s
	WHERE status = 'SENDING'
	  AND updated_at < now() - $1::int * INTERVAL '1 second'
	  AND (locked_until IS NULL OR locked_until < now())
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
RETURNING d.id, d.notification_id::text,
	COALESCE((SELECT n.trace_context FROM notifications n WHERE n.id = d.notification_id), '')`, table)
}
