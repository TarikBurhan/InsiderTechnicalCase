package queries

const settingColumns = `key, value, description, updated_at`

// ListSettings returns every account setting.
const ListSettings = `SELECT ` + settingColumns + ` FROM account_settings ORDER BY key`

// GetSetting returns one account setting.
const GetSetting = `SELECT ` + settingColumns + ` FROM account_settings WHERE key = $1`

// UpdateSetting changes a setting. The trg_account_settings_apply trigger
// validates the time zone and applies it as the database default.
const UpdateSetting = `UPDATE account_settings SET value = $2 WHERE key = $1 RETURNING ` + settingColumns

// TimezoneExists checks a name against PostgreSQL's time zone list.
const TimezoneExists = `SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)`

// DatabaseTime returns the session zone, the database default zone and now().
const DatabaseTime = `
SELECT current_setting('TimeZone'),
       COALESCE((
           SELECT split_part(cfg, '=', 2)
           FROM pg_db_role_setting s, unnest(s.setconfig) AS cfg
           WHERE s.setdatabase = (SELECT oid FROM pg_database WHERE datname = current_database())
             AND s.setrole = 0 AND cfg ILIKE 'timezone=%'
       ), current_setting('TimeZone')),
       now()`
