package dto

import "encoding/json"

// Notification is the subset of a notification row the Manager works with.
type Notification struct {
	ID                 string
	CorrelationID      string
	Channel            string
	Priority           string
	Status             string
	AudienceType       string
	AudienceSQL        *string
	Recipients         json.RawMessage
	Content            json.RawMessage
	CustomizedContent  bool
	ProcessingAttempts int
	TraceContext       string // W3C trace context of the API request
}

// CompletedNotification is returned when the finalizer completes a notification.
type CompletedNotification struct {
	ID            string
	CorrelationID string
	TraceContext  string
}
