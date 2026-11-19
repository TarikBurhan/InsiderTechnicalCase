DROP INDEX IF EXISTS idx_notification_logs_trace;
ALTER TABLE notification_logs DROP COLUMN IF EXISTS trace_id;
ALTER TABLE notifications DROP COLUMN IF EXISTS trace_context;
