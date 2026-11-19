// Package queries contains every SQL statement used by the Manager.
package queries

import "fmt"

const notificationColumns = `n.id::text, COALESCE(n.correlation_id, ''), n.channel, n.priority, n.status, n.audience_type,
	n.audience_sql, n.recipients, n.content, n.customized_content, n.processing_attempts, COALESCE(n.trace_context, '')`

// ClaimNewNotifications atomically moves due NEW notifications to PROCESSING,
// highest priority / oldest first. Immediate and scheduled notifications are
// read through separate partial indexes (idx_notifications_pickup_now and
// idx_notifications_pickup_scheduled), so scheduled notifications whose
// scheduled_at is still in the future are never scanned. FOR UPDATE SKIP
// LOCKED lets several Manager replicas poll concurrently without ever
// claiming the same notification twice.
const ClaimNewNotifications = `
WITH immediate AS (
	SELECT id, priority_rank, created_at FROM notifications
	WHERE status = 'NEW' AND scheduled_at IS NULL
	ORDER BY priority_rank, created_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
), due AS (
	SELECT id, priority_rank, created_at FROM notifications
	WHERE status = 'NEW' AND scheduled_at IS NOT NULL AND scheduled_at <= now()
	ORDER BY scheduled_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
), picked AS (
	SELECT id FROM (SELECT * FROM immediate UNION ALL SELECT * FROM due) AS c
	ORDER BY priority_rank, created_at
	LIMIT $1
)
UPDATE notifications n
SET status = 'PROCESSING', processing_started_at = now(), heartbeat_at = now(),
    processing_attempts = n.processing_attempts + 1, updated_at = now()
FROM picked
WHERE n.id = picked.id
RETURNING ` + notificationColumns

// ClaimStaleProcessing takes over PROCESSING notifications whose owner stopped
// sending heartbeats (e.g. the Manager crashed mid-way).
const ClaimStaleProcessing = `
UPDATE notifications n
SET heartbeat_at = now(), processing_attempts = n.processing_attempts + 1, updated_at = now()
WHERE n.id IN (
	SELECT id FROM notifications
	WHERE status = 'PROCESSING' AND (heartbeat_at IS NULL OR heartbeat_at < now() - $2::int * INTERVAL '1 second')
	ORDER BY priority_rank, created_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
)
RETURNING ` + notificationColumns

// Heartbeat marks a PROCESSING notification as alive.
const Heartbeat = `UPDATE notifications SET heartbeat_at = now() WHERE id = $1::uuid AND status = 'PROCESSING'`

// GetStatus returns the current status of a notification.
const GetStatus = `SELECT status FROM notifications WHERE id = $1::uuid`

// MarkSending records that every batch was produced.
const MarkSending = `
UPDATE notifications
SET status = 'SENDING', total_recipients = $2, sending_started_at = now(), heartbeat_at = NULL, updated_at = now()
WHERE id = $1::uuid AND status = 'PROCESSING'`

// MarkError fails a notification.
const MarkError = `
UPDATE notifications
SET status = 'ERROR', error_message = $2, completed_at = now(), heartbeat_at = NULL, updated_at = now()
WHERE id = $1::uuid AND status IN ('PROCESSING', 'SENDING')`

// LoadNotifications loads notifications by id (used by the retry scheduler).
const LoadNotifications = `SELECT ` + notificationColumns + ` FROM notifications n WHERE n.id = ANY($1::text[]::uuid[])`

// CompleteIfDone completes one SENDING notification once none of its
// deliveries is pending anymore.
func CompleteIfDone(deliveryTable string) string {
	return fmt.Sprintf(`
UPDATE notifications n
SET status = 'COMPLETE', completed_at = now(), updated_at = now()
WHERE n.id = $1::uuid AND n.status = 'SENDING'
  AND NOT EXISTS (SELECT 1 FROM %s d WHERE d.notification_id = n.id AND d.status IN ('SENDING', 'RETRY'))`, deliveryTable)
}

// CompleteFinished completes every finished SENDING notification of a channel.
func CompleteFinished(deliveryTable string) string {
	return fmt.Sprintf(`
UPDATE notifications n
SET status = 'COMPLETE', completed_at = now(), updated_at = now()
WHERE n.status = 'SENDING' AND n.channel = $1
  AND NOT EXISTS (SELECT 1 FROM %s d WHERE d.notification_id = n.id AND d.status IN ('SENDING', 'RETRY'))
RETURNING n.id::text, COALESCE(n.correlation_id, ''), COALESCE(n.trace_context, '')`, deliveryTable)
}
