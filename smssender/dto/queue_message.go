// Package dto contains the sender's data transfer objects.
package dto

import (
	"encoding/json"
	"time"
)

// MessageContent is the content carried by queue messages.
type MessageContent struct {
	Subject  string          `json:"subject,omitempty"`
	Title    string          `json:"title,omitempty"`
	Body     string          `json:"body"`
	IsHTML   bool            `json:"is_html,omitempty"`
	ImageURL string          `json:"image_url,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// QueueRecipient is one recipient. Content is set when it was personalised.
type QueueRecipient struct {
	DeliveryID int64           `json:"delivery_id"`
	ContactID  *int64          `json:"contact_id,omitempty"`
	To         string          `json:"to"`
	Attempt    int             `json:"attempt,omitempty"` // set for retries
	Content    *MessageContent `json:"content,omitempty"`
}

// QueueMessage is a batch of at most 100 recipients.
type QueueMessage struct {
	MessageID      string           `json:"message_id"`
	NotificationID string           `json:"notification_id"`
	Channel        string           `json:"channel"`
	Priority       string           `json:"priority"`
	CorrelationID  string           `json:"correlation_id,omitempty"`
	BatchNo        int              `json:"batch_no"`
	Attempt        int              `json:"attempt"`
	Customized     bool             `json:"customized"`
	Content        MessageContent   `json:"content"`
	Recipients     []QueueRecipient `json:"recipients"`
	CreatedAt      time.Time        `json:"created_at"`
}

// AttemptFor returns the attempt number of r (its own for retries, else the
// batch attempt).
func (m QueueMessage) AttemptFor(r QueueRecipient) int {
	if r.Attempt > 0 {
		return r.Attempt
	}
	if m.Attempt > 0 {
		return m.Attempt
	}
	return 1
}

// ContentFor returns the personalised content of r or the batch content.
func (m QueueMessage) ContentFor(r QueueRecipient) MessageContent {
	if r.Content != nil {
		return *r.Content
	}
	return m.Content
}
