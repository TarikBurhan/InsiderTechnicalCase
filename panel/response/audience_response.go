package response

import (
	"encoding/json"
	"time"

	"notifier/panel/dto"
)

// AudienceGroupResponse is a saved audience group.
type AudienceGroupResponse struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	SourceType   string          `json:"source_type"`
	Definition   json.RawMessage `json:"definition,omitempty"`
	SQLQuery     string          `json:"sql_query"`
	ContactCount int64           `json:"contact_count"`
	ValidatedAt  time.Time       `json:"validated_at"`
	CreatedAt    time.Time       `json:"created_at"`
}

// AudienceValidationResponse is the result of an audience dry-run.
type AudienceValidationResponse struct {
	Valid       bool    `json:"valid"`
	Type        string  `json:"type"`
	Count       int64   `json:"count"`
	ResolvedSQL *string `json:"resolved_sql,omitempty"`
}

// FromAudienceGroup maps a DTO.
func FromAudienceGroup(g dto.AudienceGroup) AudienceGroupResponse {
	return AudienceGroupResponse{
		ID: g.ID, Name: g.Name, Description: g.Description, SourceType: g.SourceType, Definition: g.Definition,
		SQLQuery: g.SQLQuery, ContactCount: g.ContactCount, ValidatedAt: g.ValidatedAt, CreatedAt: g.CreatedAt,
	}
}
