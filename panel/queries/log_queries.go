package queries

// InsertLog writes an event to notification_logs.
const InsertLog = `
INSERT INTO notification_logs (notification_id, channel, service, level, event, message, correlation_id, details)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)`

// ListLogs pages through the logs of a notification.
const ListLogs = `
SELECT id, notification_id::text, delivery_id, channel, service, level, event, message, correlation_id, details, created_at
FROM notification_logs
WHERE notification_id = $1::uuid
ORDER BY created_at, id
LIMIT $2 OFFSET $3`

// CountLogs counts the logs of a notification.
const CountLogs = `SELECT count(*) FROM notification_logs WHERE notification_id = $1::uuid`
