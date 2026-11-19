-- =====================================================================
-- 000003 - account settings (key/value) and the account time zone
-- =====================================================================

CREATE TABLE account_settings (
    key          TEXT        PRIMARY KEY,
    value        TEXT        NOT NULL,
    description  TEXT,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Validates settings at the database level and keeps the database default
-- time zone equal to the account time zone, whoever updates the row.
CREATE OR REPLACE FUNCTION apply_account_setting()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    IF NEW.key = 'timezone' THEN
        IF NOT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = NEW.value) THEN
            RAISE EXCEPTION 'invalid timezone: %', NEW.value USING ERRCODE = '22023';
        END IF;
        EXECUTE format('ALTER DATABASE %I SET timezone = %L', current_database(), NEW.value);
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_account_settings_apply
BEFORE INSERT OR UPDATE ON account_settings
FOR EACH ROW EXECUTE FUNCTION apply_account_setting();

INSERT INTO account_settings (key, value, description) VALUES
    ('timezone', 'Europe/Istanbul',
     'IANA time zone of the account. Used by database sessions, every service and API date handling (scheduled_at, filters, responses).');
