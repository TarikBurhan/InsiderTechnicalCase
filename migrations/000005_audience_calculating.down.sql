DROP INDEX IF EXISTS idx_notifications_calculating;
UPDATE notifications SET status = 'ERROR', error_message = 'audience calculation rolled back'
WHERE status = 'AUDIENCE_CALCULATING';
ALTER TABLE notifications DROP COLUMN IF EXISTS audience_count;
ALTER TABLE notifications DROP CONSTRAINT notifications_status_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_status_check CHECK (
    status IN ('NEW', 'PROCESSING', 'SENDING', 'COMPLETE', 'ERROR', 'CANCELLED')
);
