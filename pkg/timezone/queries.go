package timezone

// LoadAccountTimezone reads the account time zone from account_settings.
const LoadAccountTimezone = `SELECT value FROM account_settings WHERE key = 'timezone'`

// SetSessionTimezone sets the time zone of the current session.
const SetSessionTimezone = `SELECT set_config('TimeZone', $1, false)`

// VerifySession returns the session time zone and the database clock.
const VerifySession = `SELECT current_setting('TimeZone'), now()`
