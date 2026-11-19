package dto

import (
	"encoding/json"
	"time"
)

// MessageContent is the channel agnostic content snapshot stored on a
// notification. Only the fields relevant to the channel are filled.
type MessageContent struct {
	Subject  string          `json:"subject,omitempty"`
	Title    string          `json:"title,omitempty"`
	Body     string          `json:"body"`
	IsHTML   bool            `json:"is_html,omitempty"`
	ImageURL string          `json:"image_url,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// Notification mirrors a row of the notifications table.
type Notification struct {
	ID                 string
	BatchID            *string
	IdempotencyKey     *string
	CorrelationID      *string
	Channel            string
	Priority           string
	Status             string
	AudienceType       string
	AudienceGroupID    *int64
	AudienceDefinition json.RawMessage
	AudienceSQL        *string
	Recipients         json.RawMessage
	ContentID          *int64
	Content            json.RawMessage
	CustomizedContent  bool
	ScheduledAt        *time.Time
	TotalRecipients    int
	ErrorMessage       *string
	ProcessingStarted  *time.Time
	SendingStarted     *time.Time
	CompletedAt        *time.Time
	CancelledAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	TraceContext       string
	AudienceCount      *int64
}

// NewNotification is the data needed to insert a notification.
type NewNotification struct {
	BatchID            *string
	IdempotencyKey     *string
	CorrelationID      string
	Channel            string
	Priority           string
	AudienceType       string
	AudienceGroupID    *int64
	AudienceDefinition json.RawMessage
	AudienceSQL        *string
	Recipients         json.RawMessage
	ContentID          *int64
	Content            json.RawMessage
	CustomizedContent  bool
	ScheduledAt        *time.Time
	TraceContext       string
	Status             string // NEW, or AUDIENCE_CALCULATING when counting timed out
	AudienceCount      *int64
}

// NotificationFilter is used to list notifications.
type NotificationFilter struct {
	Status   string
	Channel  string
	Priority string
	BatchID  string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// DeliveryStats aggregates the delivery rows of one notification.
type DeliveryStats struct {
	Total     int `json:"total"`
	Sending   int `json:"sending"`
	Sent      int `json:"sent"`
	Retry     int `json:"retry"`
	Error     int `json:"error"`
	Cancelled int `json:"cancelled"`
}

// Delivery mirrors a row of a <channel>_deliveries table.
type Delivery struct {
	ID                int64
	NotificationID    string
	ContactID         *int64
	Recipient         string
	BatchNo           int
	Status            string
	RetryCount        int
	NextRetryAt       *time.Time
	ProviderMessageID *string
	LastError         *string
	SentAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NotificationLog mirrors a row of notification_logs.
type NotificationLog struct {
	ID             int64
	NotificationID *string
	DeliveryID     *int64
	Channel        *string
	Service        string
	Level          string
	Event          string
	Message        *string
	CorrelationID  *string
	TraceID        *string
	Details        json.RawMessage
	CreatedAt      time.Time
}

// AudienceCalculation is a notification whose audience is counted in the
// background.
type AudienceCalculation struct {
	NotificationID string
	Channel        string
	CorrelationID  string
	AudienceSQL    string
	TraceContext   string
}

// StatusEvent is broadcast to WebSocket clients when a status changes.
type StatusEvent struct {
	ID              string  `json:"id"`
	BatchID         *string `json:"batch_id"`
	Channel         string  `json:"channel"`
	Status          string  `json:"status"`
	TotalRecipients int     `json:"total_recipients"`
	ErrorMessage    *string `json:"error_message"`
	UpdatedAt       string  `json:"updated_at"`
}
