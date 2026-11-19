DROP TRIGGER IF EXISTS trg_account_settings_apply ON account_settings;
DROP FUNCTION IF EXISTS apply_account_setting();
DROP TABLE IF EXISTS account_settings;
DO $$
BEGIN
    EXECUTE format('ALTER DATABASE %I RESET timezone', current_database());
END
$$;
