package dto

import "time"

// SettingTimezone is the key of the account time zone setting.
const SettingTimezone = "timezone"

// Setting mirrors a row of account_settings.
type Setting struct {
	Key         string
	Value       string
	Description *string
	UpdatedAt   time.Time
}

// DatabaseTime is what PostgreSQL reports about time zones and its clock.
type DatabaseTime struct {
	SessionTimezone  string
	DatabaseTimezone string
	Now              time.Time
}
