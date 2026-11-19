package queries

import (
	"fmt"

	"notifier/customizer/dto"
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

// MarkDeliveriesFailed permanently fails deliveries that cannot be
// personalised (contact deleted, rendered content over the channel limit).
func MarkDeliveriesFailed(table string) string {
	return fmt.Sprintf(`
UPDATE %s d
SET status = 'ERROR', last_error = u.reason, locked_until = NULL, next_retry_at = NULL, updated_at = now()
FROM unnest($1::bigint[], $2::text[]) AS u(id, reason)
WHERE d.id = u.id AND d.status = 'SENDING'`, table)
}
