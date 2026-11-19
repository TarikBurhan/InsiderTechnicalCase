// Package queries contains every SQL statement used by the Email sender.
package queries

// ClaimDeliveries locks deliveries for sending. Only rows still in SENDING
// and not leased by another consumer are returned, which makes processing
// idempotent: a redelivered or duplicated queue message cannot send the
// same delivery twice, and cancelled deliveries are skipped.
const ClaimDeliveries = `
UPDATE email_deliveries
SET locked_until = now() + $2::int * INTERVAL '1 second', updated_at = now()
WHERE id = ANY($1::bigint[])
  AND status = 'SENDING'
  AND (locked_until IS NULL OR locked_until < now())
RETURNING id`

// MarkSent records successful deliveries with the provider message id.
const MarkSent = `
UPDATE email_deliveries d
SET status = 'SENT', provider_message_id = NULLIF(u.message_id, ''), sent_at = now(),
    last_error = NULL, locked_until = NULL, next_retry_at = NULL, updated_at = now()
FROM unnest($1::bigint[], $2::text[]) AS u(id, message_id)
WHERE d.id = u.id`

// MarkFailed increments retry_count. Deliveries go to RETRY (next attempt
// after $3 seconds) until retry_count reaches $2 or the error is permanent,
// then to ERROR.
const MarkFailed = `
UPDATE email_deliveries d
SET retry_count   = d.retry_count + 1,
    status        = CASE WHEN u.permanent OR d.retry_count + 1 >= $2 THEN 'ERROR' ELSE 'RETRY' END,
    next_retry_at = CASE WHEN u.permanent OR d.retry_count + 1 >= $2 THEN NULL
                         ELSE now() + $3::int * INTERVAL '1 second' END,
    last_error    = u.error,
    locked_until  = NULL,
    updated_at    = now()
FROM unnest($1::bigint[], $4::text[], $5::bool[]) AS u(id, error, permanent)
WHERE d.id = u.id AND d.status = 'SENDING'
RETURNING d.id, d.status, d.retry_count, COALESCE(d.last_error, '')`
