-- =====================================================================
-- 000006 - indexes for polling and data retention
-- =====================================================================

-- Polling: immediate and scheduled NEW notifications get separate indexes,
-- so future scheduled notifications are never scanned by the poller.
DROP INDEX IF EXISTS idx_notifications_pickup;
CREATE INDEX idx_notifications_pickup_now
    ON notifications (priority_rank, created_at)
    WHERE status = 'NEW' AND scheduled_at IS NULL;
CREATE INDEX idx_notifications_pickup_scheduled
    ON notifications (scheduled_at)
    WHERE status = 'NEW' AND scheduled_at IS NOT NULL;

-- Retention: finished notifications by the time they finished.
CREATE INDEX idx_notifications_finished
    ON notifications ((COALESCE(completed_at, cancelled_at, updated_at)))
    WHERE status IN ('COMPLETE', 'ERROR', 'CANCELLED');
