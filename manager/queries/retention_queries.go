package queries

import "fmt"

// finishedBefore matches notifications that finished more than $1 days ago.
// It uses the idx_notifications_finished expression index.
const finishedBefore = `n.status IN ('COMPLETE', 'ERROR', 'CANCELLED')
	AND COALESCE(n.completed_at, n.cancelled_at, n.updated_at) < now() - $1::int * INTERVAL '1 day'`

// DeleteOldDeliveries deletes up to $2 delivery rows of notifications that
// finished more than $1 days ago. Small batches keep locks and WAL bursts
// short; SKIP LOCKED lets several Managers run retention at the same time.
func DeleteOldDeliveries(table string) string {
	return fmt.Sprintf(`
DELETE FROM %[1]s WHERE id IN (
	SELECT d.id FROM %[1]s d
	JOIN notifications n ON n.id = d.notification_id
	WHERE %[2]s
	LIMIT $2
	FOR UPDATE OF d SKIP LOCKED
)`, table, finishedBefore)
}

// DeleteOldNotifications deletes up to $2 finished notifications of channel
// $3 whose deliveries are already gone.
func DeleteOldNotifications(table string) string {
	return fmt.Sprintf(`
DELETE FROM notifications WHERE id IN (
	SELECT n.id FROM notifications n
	WHERE %[2]s
	  AND n.channel = $3
	  AND NOT EXISTS (SELECT 1 FROM %[1]s d WHERE d.notification_id = n.id)
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)`, table, finishedBefore)
}

// DeleteOldLogs deletes up to $2 audit log rows older than $1 days, except
// the ones of notifications that are still in progress (e.g. scheduled far
// in the future).
const DeleteOldLogs = `
DELETE FROM notification_logs WHERE id IN (
	SELECT l.id FROM notification_logs l
	WHERE l.created_at < now() - $1::int * INTERVAL '1 day'
	  AND NOT EXISTS (
		SELECT 1 FROM notifications n
		WHERE n.id = l.notification_id
		  AND n.status IN ('AUDIENCE_CALCULATING', 'NEW', 'PROCESSING', 'SENDING')
	  )
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)`
