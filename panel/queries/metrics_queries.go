package queries

import "fmt"

// NotificationStatusCounts counts notifications per status in the last 24h.
const NotificationStatusCounts = `
SELECT status, count(*) FROM notifications
WHERE created_at > now() - INTERVAL '24 hours'
GROUP BY status`

// ChannelDeliveryStats aggregates delivery outcomes of a channel for rows
// touched in the last $1 seconds. Latency = time from being queued (row
// creation) until the provider accepted the message.
func ChannelDeliveryStats(table string) string {
	return fmt.Sprintf(`
SELECT
	count(*) FILTER (WHERE status = 'SENT'),
	count(*) FILTER (WHERE status = 'ERROR'),
	count(*) FILTER (WHERE status = 'RETRY'),
	count(*) FILTER (WHERE status = 'SENDING'),
	COALESCE(avg(EXTRACT(EPOCH FROM (sent_at - created_at)) * 1000) FILTER (WHERE status = 'SENT'), 0)::float8,
	COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (sent_at - created_at)) * 1000)
		FILTER (WHERE status = 'SENT'), 0)::float8
FROM %s
WHERE updated_at > now() - $1::int * INTERVAL '1 second'`, table)
}
