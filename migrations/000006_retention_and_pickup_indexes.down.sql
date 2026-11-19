DROP INDEX IF EXISTS idx_notifications_finished;
DROP INDEX IF EXISTS idx_notifications_pickup_scheduled;
DROP INDEX IF EXISTS idx_notifications_pickup_now;
CREATE INDEX idx_notifications_pickup ON notifications (priority_rank, created_at) WHERE status = 'NEW';
