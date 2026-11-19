package queries

// LogTable and LogColumns are used with COPY to bulk insert delivery logs.
const LogTable = "notification_logs"

// LogColumns is the column order used by the bulk copy.
var LogColumns = []string{"notification_id", "delivery_id", "channel", "service", "level", "event", "message", "correlation_id", "details", "trace_id"}

// InsertLog writes a single notification log entry.
const InsertLog = `
INSERT INTO notification_logs (notification_id, delivery_id, channel, service, level, event, message, correlation_id, details, trace_id)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''))`
