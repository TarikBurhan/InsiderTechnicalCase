package queries

// InsertLog writes an event to notification_logs.
const InsertLog = `
INSERT INTO notification_logs (notification_id, channel, service, level, event, message, correlation_id, details, trace_id)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''))`
