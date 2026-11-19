package request

// UpdateSettingRequest changes the value of an account setting.
type UpdateSettingRequest struct {
	Value string `json:"value"`
}
