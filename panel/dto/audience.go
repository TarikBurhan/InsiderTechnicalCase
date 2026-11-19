package dto

import (
	"encoding/json"
	"time"
)

// AudienceGroup mirrors a row of audience_groups.
type AudienceGroup struct {
	ID           int64
	Name         string
	Description  *string
	SourceType   string
	Definition   json.RawMessage
	SQLQuery     string
	ContactCount int64
	ValidatedAt  time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ResolvedAudience is the outcome of validating an audience request.
type ResolvedAudience struct {
	// Pending is true when counting did not finish within the API timeout;
	// the definition is valid and the count continues in the background.
	Pending    bool
	Type       string
	SQL        *string
	GroupID    *int64
	Recipients []string
	Definition json.RawMessage
	Count      int64
}

// TableSchema maps table name -> column name -> data type for the tables an
// audience may reference.
type TableSchema map[string]map[string]string

// HasColumn reports whether table has column.
func (s TableSchema) HasColumn(table, column string) bool {
	cols, ok := s[table]
	if !ok {
		return false
	}
	_, ok = cols[column]
	return ok
}
