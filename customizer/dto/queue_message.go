// Package dto contains the Customizer's data transfer objects.
package dto

import (
	"encoding/json"
	"time"
)

// Channels.
const (
	ChannelSMS   = "sms"
	ChannelEmail = "email"
	ChannelPush  = "push"
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

// QueueRecipient is one recipient inside a queue message. Content is the
// personalised content filled in by the Customizer.
type QueueRecipient struct {
	DeliveryID int64           `json:"delivery_id"`
	ContactID  *int64          `json:"contact_id,omitempty"`
	To         string          `json:"to"`
	Attempt    int             `json:"attempt,omitempty"`
	Content    *MessageContent `json:"content,omitempty"`
}

// QueueMessage is a batch of recipients.
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

// FailedRecipient is a recipient whose content could not be personalised.
type FailedRecipient struct {
	DeliveryID int64
	Reason     string
}

// ContactVariables maps master_contact column name -> rendered value.
type ContactVariables map[string]string
