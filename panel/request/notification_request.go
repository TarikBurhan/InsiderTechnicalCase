// Package request contains the HTTP request bodies / query parameters the
// Panel API accepts.
package request

import (
	"time"
)

// CreateNotificationRequest creates a single notification request.
type CreateNotificationRequest struct {
	Channel           string          `json:"channel"`
	Priority          string          `json:"priority,omitempty"`
	IdempotencyKey    string          `json:"idempotency_key,omitempty"`
	ScheduledAt       *FlexibleTime   `json:"scheduled_at,omitempty"`
	Audience          AudienceRequest `json:"audience"`
	ContentID         *int64          `json:"content_id,omitempty"`
	Content           *ContentPayload `json:"content,omitempty"`
	CustomizedContent bool            `json:"customized_content"`
}

// BatchCreateNotificationRequest creates up to N notifications at once.
type BatchCreateNotificationRequest struct {
	Notifications []CreateNotificationRequest `json:"notifications"`
}

// ContentPayload is inline content sent with a notification request.
type ContentPayload struct {
	Subject  string         `json:"subject,omitempty"`
	Title    string         `json:"title,omitempty"`
	Body     string         `json:"body"`
	IsHTML   bool           `json:"is_html,omitempty"`
	ImageURL string         `json:"image_url,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// ListNotificationsQuery holds list filters parsed from the query string.
type ListNotificationsQuery struct {
	Status   string
	Channel  string
	Priority string
	BatchID  string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

// PageQuery is a simple pagination query.
type PageQuery struct {
	Page     int
	PageSize int
	Status   string
}
