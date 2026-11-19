package request

// AudienceRequest selects who receives a notification. Exactly one of the
// type specific fields is used depending on Type:
//   - SQL:    raw SELECT returning an `id` column of master_contact
//   - FILTER: column filters on master_contact (optionally joined table)
//   - GROUP:  id of a saved, already validated audience group
//   - DIRECT: explicit list of addresses (phone / email / device token)
type AudienceRequest struct {
	Type       string          `json:"type"`
	SQL        string          `json:"sql,omitempty"`
	Filter     *AudienceFilter `json:"filter,omitempty"`
	GroupID    *int64          `json:"group_id,omitempty"`
	Recipients []string        `json:"recipients,omitempty"`
}

// AudienceFilter builds an audience from column conditions.
// Table is optional; when set to a whitelisted table having a contact_id
// column it is joined to master_contact. Condition fields can be written as
// "column" (master_contact) or "table.column".
type AudienceFilter struct {
	Table      string              `json:"table,omitempty"`
	Match      string              `json:"match,omitempty"` // all (AND, default) | any (OR)
	Conditions []AudienceCondition `json:"conditions"`
}

// AudienceCondition is a single filter condition.
type AudienceCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

// CreateAudienceGroupRequest saves a validated audience group.
type CreateAudienceGroupRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"` // SQL | FILTER
	SQL         string          `json:"sql,omitempty"`
	Filter      *AudienceFilter `json:"filter,omitempty"`
}

// ValidateAudienceRequest dry-runs an audience definition.
type ValidateAudienceRequest struct {
	Channel  string          `json:"channel"`
	Audience AudienceRequest `json:"audience"`
}
