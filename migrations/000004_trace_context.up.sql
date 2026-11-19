-- =====================================================================
-- 000004 - keep the distributed trace across the database hop
-- =====================================================================

-- W3C trace context (traceparent/tracestate as JSON) of the API request that
-- created the notification. The Manager, retry scheduler, finalizer and
-- sweeper continue this trace, so a notification is one trace end to end.
ALTER TABLE notifications ADD COLUMN trace_context TEXT;

-- Trace id of the span that wrote each audit log entry.
ALTER TABLE notification_logs ADD COLUMN trace_id TEXT;
CREATE INDEX idx_notification_logs_trace ON notification_logs (trace_id) WHERE trace_id IS NOT NULL;
