package queries

// ClaimAudienceCalculations takes AUDIENCE_CALCULATING notifications that
// nobody is working on (no heartbeat within the lease). SKIP LOCKED lets
// several Panel instances share the work; a crashed instance's work is
// picked up again once its lease expires.
const ClaimAudienceCalculations = `
UPDATE notifications n
SET heartbeat_at = now(), updated_at = now()
WHERE n.id IN (
	SELECT id FROM notifications
	WHERE status = 'AUDIENCE_CALCULATING'
	  AND (heartbeat_at IS NULL OR heartbeat_at < now() - $2::int * INTERVAL '1 second')
	ORDER BY created_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
)
RETURNING n.id::text, n.channel, COALESCE(n.correlation_id, ''), COALESCE(n.audience_sql, ''), COALESCE(n.trace_context, '')`

// HeartbeatAudienceCalculation extends the lease of a running calculation.
const HeartbeatAudienceCalculation = `
UPDATE notifications SET heartbeat_at = now()
WHERE id = $1::uuid AND status = 'AUDIENCE_CALCULATING'`

// CompleteAudienceCalculation stores the count and releases the
// notification to the Manager (NEW). A cancelled notification is untouched.
const CompleteAudienceCalculation = `
UPDATE notifications
SET status = 'NEW', audience_count = $2, heartbeat_at = NULL, updated_at = now()
WHERE id = $1::uuid AND status = 'AUDIENCE_CALCULATING'`

// FailAudienceCalculation moves the notification to ERROR.
const FailAudienceCalculation = `
UPDATE notifications
SET status = 'ERROR', error_message = $2, completed_at = now(), heartbeat_at = NULL, updated_at = now()
WHERE id = $1::uuid AND status = 'AUDIENCE_CALCULATING'`
