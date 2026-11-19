// Package validation holds the Panel's pure validation logic: audience SQL
// sanitizing, filter -> SQL building, content and recipient validation.
package validation

import "strings"

// FieldError describes a problem with one field.
type FieldError struct {
	Field   string
	Message string
}

// Errors is a list of field errors and implements error.
type Errors []FieldError

func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for _, fe := range e {
		parts = append(parts, fe.Field+": "+fe.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Add appends a field error.
func (e *Errors) Add(field, message string) {
	*e = append(*e, FieldError{Field: field, Message: message})
}

// Err returns nil when there are no errors.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// New creates an Errors value with a single error.
func New(field, message string) Errors { return Errors{{Field: field, Message: message}} }

// Prefix returns a copy of e with every field prefixed (e.g. "notifications[3].").
func (e Errors) Prefix(prefix string) Errors {
	out := make(Errors, len(e))
	for i, fe := range e {
		out[i] = FieldError{Field: prefix + fe.Field, Message: fe.Message}
	}
	return out
}
