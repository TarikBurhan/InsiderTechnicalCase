package response

import (
	"time"

	"notifier/panel/dto"
)

// SettingResponse is an account setting.
type SettingResponse struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description *string   `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpdateSettingResponse is returned after a setting changed.
type UpdateSettingResponse struct {
	SettingResponse
	Note string `json:"note,omitempty"`
}

// TimeStatusResponse shows how the account time zone is applied.
type TimeStatusResponse struct {
	AccountTimezone         string    `json:"account_timezone"`
	ServiceTimezone         string    `json:"service_timezone"`
	ContainerTZ             string    `json:"container_tz"`
	DatabaseTimezone        string    `json:"database_timezone"`
	DatabaseSessionTimezone string    `json:"database_session_timezone"`
	ServiceNow              time.Time `json:"service_now"`
	DatabaseNow             time.Time `json:"database_now"`
	ClockDriftMs            int64     `json:"clock_drift_ms"`
	Consistent              bool      `json:"consistent"`
	Problems                []string  `json:"problems,omitempty"`
}

// FromSetting maps a DTO.
func FromSetting(s dto.Setting) SettingResponse {
	return SettingResponse{Key: s.Key, Value: s.Value, Description: s.Description, UpdatedAt: s.UpdatedAt}
}
