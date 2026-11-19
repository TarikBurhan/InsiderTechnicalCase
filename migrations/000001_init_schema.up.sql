-- =====================================================================
-- 000001 - core schema for the event-driven notification system
-- =====================================================================

-- ---------------------------------------------------------------------
-- Contacts (audience source)
-- ---------------------------------------------------------------------
CREATE TABLE master_contact (
    id            BIGSERIAL PRIMARY KEY,
    first_name    TEXT        NOT NULL,
    last_name     TEXT        NOT NULL,
    email         TEXT,
    phone         TEXT,
    device_token  TEXT,
    city          TEXT,
    country       TEXT,
    language      TEXT        NOT NULL DEFAULT 'tr',
    gender        TEXT,
    birth_date    DATE,
    loyalty_tier  TEXT,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_master_contact_city ON master_contact (city);
CREATE INDEX idx_master_contact_country ON master_contact (country);
CREATE INDEX idx_master_contact_loyalty ON master_contact (loyalty_tier);

-- Example of an additional "special" table that FILTER audiences can join.
CREATE TABLE contact_purchases (
    id               BIGSERIAL PRIMARY KEY,
    contact_id       BIGINT        NOT NULL REFERENCES master_contact (id) ON DELETE CASCADE,
    product_category TEXT          NOT NULL,
    amount           NUMERIC(12,2) NOT NULL,
    purchased_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_contact_purchases_contact ON contact_purchases (contact_id);

-- ---------------------------------------------------------------------
-- Saved, pre-validated audience groups
-- ---------------------------------------------------------------------
CREATE TABLE audience_groups (
    id             BIGSERIAL PRIMARY KEY,
    name           TEXT        NOT NULL UNIQUE,
    description    TEXT,
    source_type    TEXT        NOT NULL CHECK (source_type IN ('SQL', 'FILTER')),
    definition     JSONB,
    sql_query      TEXT        NOT NULL,
    contact_count  BIGINT      NOT NULL DEFAULT 0,
    validated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- Pre-validated contents / templates per channel
-- ---------------------------------------------------------------------
CREATE TABLE sms_content (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    body        TEXT        NOT NULL,
    variables   TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE email_content (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    subject     TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    is_html     BOOLEAN     NOT NULL DEFAULT TRUE,
    variables   TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE push_content (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    title       TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    image_url   TEXT,
    data        JSONB,
    variables   TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- Notification requests (one row per request coming from Panel)
--   NEW -> PROCESSING -> SENDING -> COMPLETE
--   any -> ERROR, NEW/PROCESSING/SENDING -> CANCELLED
-- ---------------------------------------------------------------------
CREATE TABLE notifications (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id              UUID,
    idempotency_key       TEXT        UNIQUE,
    correlation_id        TEXT,
    channel               TEXT        NOT NULL CHECK (channel IN ('sms', 'email', 'push')),
    priority              TEXT        NOT NULL DEFAULT 'normal' CHECK (priority IN ('high', 'normal', 'low')),
    priority_rank         SMALLINT    GENERATED ALWAYS AS (
                              CASE priority WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END
                          ) STORED,
    status                TEXT        NOT NULL DEFAULT 'NEW'
                          CHECK (status IN ('NEW', 'PROCESSING', 'SENDING', 'COMPLETE', 'ERROR', 'CANCELLED')),
    audience_type         TEXT        NOT NULL CHECK (audience_type IN ('SQL', 'FILTER', 'GROUP', 'DIRECT')),
    audience_group_id     BIGINT      REFERENCES audience_groups (id) ON DELETE SET NULL,
    audience_definition   JSONB,
    audience_sql          TEXT,
    recipients            JSONB,
    content_id            BIGINT,
    content               JSONB       NOT NULL,
    customized_content    BOOLEAN     NOT NULL DEFAULT FALSE,
    scheduled_at          TIMESTAMPTZ,
    total_recipients      INTEGER     NOT NULL DEFAULT 0,
    processing_attempts   INTEGER     NOT NULL DEFAULT 0,
    error_message         TEXT,
    processing_started_at TIMESTAMPTZ,
    heartbeat_at          TIMESTAMPTZ,
    sending_started_at    TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    cancelled_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_audience_payload CHECK (
        (audience_type = 'DIRECT' AND recipients IS NOT NULL) OR
        (audience_type <> 'DIRECT' AND audience_sql IS NOT NULL)
    )
);
-- Manager polling: oldest NEW rows first, by priority.
CREATE INDEX idx_notifications_pickup ON notifications (priority_rank, created_at) WHERE status = 'NEW';
CREATE INDEX idx_notifications_processing ON notifications (heartbeat_at) WHERE status = 'PROCESSING';
CREATE INDEX idx_notifications_sending ON notifications (channel) WHERE status = 'SENDING';
CREATE INDEX idx_notifications_batch ON notifications (batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_notifications_list ON notifications (created_at DESC, id);
CREATE INDEX idx_notifications_status_channel ON notifications (status, channel, created_at DESC);

-- ---------------------------------------------------------------------
-- Per-channel delivery tables (one row per audience member)
--   SENDING -> SENT | RETRY -> (SENDING ...) | ERROR | CANCELLED
-- ---------------------------------------------------------------------
CREATE TABLE sms_deliveries (
    id                  BIGSERIAL   PRIMARY KEY,
    notification_id     UUID        NOT NULL REFERENCES notifications (id) ON DELETE CASCADE,
    contact_id          BIGINT,
    recipient           TEXT        NOT NULL,
    batch_no            INTEGER     NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'SENDING'
                        CHECK (status IN ('SENDING', 'SENT', 'RETRY', 'ERROR', 'CANCELLED')),
    retry_count         SMALLINT    NOT NULL DEFAULT 0,
    next_retry_at       TIMESTAMPTZ,
    locked_until        TIMESTAMPTZ,
    provider_message_id TEXT,
    last_error          TEXT,
    sent_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_sms_deliveries_recipient UNIQUE (notification_id, recipient)
);
CREATE INDEX idx_sms_deliveries_status ON sms_deliveries (notification_id, status);
CREATE INDEX idx_sms_deliveries_retry ON sms_deliveries (next_retry_at) WHERE status = 'RETRY';
CREATE INDEX idx_sms_deliveries_stale ON sms_deliveries (updated_at) WHERE status = 'SENDING';
CREATE INDEX idx_sms_deliveries_updated ON sms_deliveries (updated_at);

CREATE TABLE email_deliveries (
    id                  BIGSERIAL   PRIMARY KEY,
    notification_id     UUID        NOT NULL REFERENCES notifications (id) ON DELETE CASCADE,
    contact_id          BIGINT,
    recipient           TEXT        NOT NULL,
    batch_no            INTEGER     NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'SENDING'
                        CHECK (status IN ('SENDING', 'SENT', 'RETRY', 'ERROR', 'CANCELLED')),
    retry_count         SMALLINT    NOT NULL DEFAULT 0,
    next_retry_at       TIMESTAMPTZ,
    locked_until        TIMESTAMPTZ,
    provider_message_id TEXT,
    last_error          TEXT,
    sent_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_email_deliveries_recipient UNIQUE (notification_id, recipient)
);
CREATE INDEX idx_email_deliveries_status ON email_deliveries (notification_id, status);
CREATE INDEX idx_email_deliveries_retry ON email_deliveries (next_retry_at) WHERE status = 'RETRY';
CREATE INDEX idx_email_deliveries_stale ON email_deliveries (updated_at) WHERE status = 'SENDING';
CREATE INDEX idx_email_deliveries_updated ON email_deliveries (updated_at);

CREATE TABLE push_deliveries (
    id                  BIGSERIAL   PRIMARY KEY,
    notification_id     UUID        NOT NULL REFERENCES notifications (id) ON DELETE CASCADE,
    contact_id          BIGINT,
    recipient           TEXT        NOT NULL,
    batch_no            INTEGER     NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'SENDING'
                        CHECK (status IN ('SENDING', 'SENT', 'RETRY', 'ERROR', 'CANCELLED')),
    retry_count         SMALLINT    NOT NULL DEFAULT 0,
    next_retry_at       TIMESTAMPTZ,
    locked_until        TIMESTAMPTZ,
    provider_message_id TEXT,
    last_error          TEXT,
    sent_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_push_deliveries_recipient UNIQUE (notification_id, recipient)
);
CREATE INDEX idx_push_deliveries_status ON push_deliveries (notification_id, status);
CREATE INDEX idx_push_deliveries_retry ON push_deliveries (next_retry_at) WHERE status = 'RETRY';
CREATE INDEX idx_push_deliveries_stale ON push_deliveries (updated_at) WHERE status = 'SENDING';
CREATE INDEX idx_push_deliveries_updated ON push_deliveries (updated_at);

-- ---------------------------------------------------------------------
-- Audit / event log written by every service
-- ---------------------------------------------------------------------
CREATE TABLE notification_logs (
    id               BIGSERIAL   PRIMARY KEY,
    notification_id  UUID,
    delivery_id      BIGINT,
    channel          TEXT,
    service          TEXT        NOT NULL,
    level            TEXT        NOT NULL DEFAULT 'INFO' CHECK (level IN ('DEBUG', 'INFO', 'WARN', 'ERROR')),
    event            TEXT        NOT NULL,
    message          TEXT,
    correlation_id   TEXT,
    details          JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_logs_notification ON notification_logs (notification_id, created_at);
CREATE INDEX idx_notification_logs_created ON notification_logs (created_at);

-- ---------------------------------------------------------------------
-- Distributed rate limiter (fixed 1s windows, database clock)
-- ---------------------------------------------------------------------
CREATE UNLOGGED TABLE rate_limit_windows (
    limiter_key   TEXT        NOT NULL,
    window_start  TIMESTAMPTZ NOT NULL,
    used          INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (limiter_key, window_start)
);

CREATE OR REPLACE FUNCTION acquire_rate_tokens(p_key TEXT, p_requested INTEGER, p_limit INTEGER)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_window  TIMESTAMPTZ := date_trunc('second', clock_timestamp());
    v_used    INTEGER;
    v_granted INTEGER;
BEGIN
    INSERT INTO rate_limit_windows (limiter_key, window_start, used)
    VALUES (p_key, v_window, 0)
    ON CONFLICT (limiter_key, window_start) DO NOTHING;

    SELECT used INTO v_used
    FROM rate_limit_windows
    WHERE limiter_key = p_key AND window_start = v_window
    FOR UPDATE;

    v_granted := GREATEST(0, LEAST(p_requested, p_limit - v_used));
    IF v_granted > 0 THEN
        UPDATE rate_limit_windows SET used = used + v_granted
        WHERE limiter_key = p_key AND window_start = v_window;
    END IF;

    -- Opportunistic cleanup of old windows.
    IF random() < 0.01 THEN
        DELETE FROM rate_limit_windows WHERE window_start < v_window - INTERVAL '1 minute';
    END IF;

    RETURN v_granted;
END;
$$;

-- ---------------------------------------------------------------------
-- Real-time status changes -> LISTEN/NOTIFY -> Panel WebSocket clients
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION notify_notification_status()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' OR NEW.status IS DISTINCT FROM OLD.status THEN
        PERFORM pg_notify('notification_status', json_build_object(
            'id', NEW.id,
            'batch_id', NEW.batch_id,
            'channel', NEW.channel,
            'status', NEW.status,
            'total_recipients', NEW.total_recipients,
            'error_message', NEW.error_message,
            'updated_at', NEW.updated_at
        )::text);
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_notifications_status
AFTER INSERT OR UPDATE OF status ON notifications
FOR EACH ROW EXECUTE FUNCTION notify_notification_status();

-- ---------------------------------------------------------------------
-- Read-only role used to run user supplied audience SQL. It may only read
-- the contact tables, so audience queries can never touch anything else.
-- The role itself is created by deploy/postgres/init (password from .env).
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audience_reader') THEN
        EXECUTE 'GRANT USAGE ON SCHEMA public TO audience_reader';
        EXECUTE 'GRANT SELECT ON master_contact, contact_purchases TO audience_reader';
    END IF;
END
$$;
