package queries

// CompleteIfDone completes the notification once no delivery is pending.
const CompleteIfDone = `
UPDATE notifications n
SET status = 'COMPLETE', completed_at = now(), updated_at = now()
WHERE n.id = $1::uuid AND n.status = 'SENDING'
  AND NOT EXISTS (
	SELECT 1 FROM email_deliveries d
	WHERE d.notification_id = n.id AND d.status IN ('SENDING', 'RETRY')
  )`
