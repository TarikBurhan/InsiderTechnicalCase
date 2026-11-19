-- =====================================================================
-- 000005 - background audience calculation
-- =====================================================================
-- When counting an audience takes longer than the API timeout the
-- notification is stored as AUDIENCE_CALCULATING and the Panel finishes the
-- calculation in the background, then moves it to NEW.

ALTER TABLE notifications DROP CONSTRAINT notifications_status_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_status_check CHECK (
    status IN ('AUDIENCE_CALCULATING', 'NEW', 'PROCESSING', 'SENDING', 'COMPLETE', 'ERROR', 'CANCELLED')
);

-- Number of master_contact rows the audience matched when it was validated.
ALTER TABLE notifications ADD COLUMN audience_count BIGINT;

CREATE INDEX idx_notifications_calculating ON notifications (created_at) WHERE status = 'AUDIENCE_CALCULATING';
