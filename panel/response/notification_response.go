package response

import (
	"encoding/json"
	"time"

	"notifier/panel/dto"
	"notifier/pkg/tracing"
)

// NotificationResponse is the public representation of a notification.
type NotificationResponse struct {
	ID                string             `json:"id"`
	BatchID           *string            `json:"batch_id,omitempty"`
	IdempotencyKey    *string            `json:"idempotency_key,omitempty"`
	CorrelationID     *string            `json:"correlation_id,omitempty"`
	TraceID           string             `json:"trace_id,omitempty"`
	Channel           string             `json:"channel"`
	Priority          string             `json:"priority"`
	Status            string             `json:"status"`
	AudienceType      string             `json:"audience_type"`
	AudienceGroupID   *int64             `json:"audience_group_id,omitempty"`
	ContentID         *int64             `json:"content_id,omitempty"`
	Content           json.RawMessage    `json:"content"`
	CustomizedContent bool               `json:"customized_content"`
	ScheduledAt       *time.Time         `json:"scheduled_at,omitempty"`
	TotalRecipients   int                `json:"total_recipients"`
	ErrorMessage      *string            `json:"error_message,omitempty"`
	Stats             *dto.DeliveryStats `json:"stats,omitempty"`
	ProcessingStarted *time.Time         `json:"processing_started_at,omitempty"`
	SendingStarted    *time.Time         `json:"sending_started_at,omitempty"`
	CompletedAt       *time.Time         `json:"completed_at,omitempty"`
	CancelledAt       *time.Time         `json:"cancelled_at,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// CreateNotificationResponse is returned after creating a notification.
type CreateNotificationResponse struct {
	NotificationResponse
	Duplicate     bool  `json:"duplicate"`
	AudienceCount int64 `json:"audience_count"`
}

// BatchCreateResponse is returned after a batch creation.
type BatchCreateResponse struct {
	BatchID       string                       `json:"batch_id"`
	Count         int                          `json:"count"`
	Notifications []CreateNotificationResponse `json:"notifications"`
}

// BatchStatusResponse summarises all notifications of a batch.
type BatchStatusResponse struct {
	BatchID       string                 `json:"batch_id"`
	Total         int                    `json:"total"`
	StatusCounts  map[string]int         `json:"status_counts"`
	Notifications []NotificationResponse `json:"notifications"`
}

// DeliveryResponse is a single delivery row.
type DeliveryResponse struct {
	ID                int64      `json:"id"`
	ContactID         *int64     `json:"contact_id,omitempty"`
	Recipient         string     `json:"recipient"`
	BatchNo           int        `json:"batch_no"`
	Status            string     `json:"status"`
	RetryCount        int        `json:"retry_count"`
	NextRetryAt       *time.Time `json:"next_retry_at,omitempty"`
	ProviderMessageID *string    `json:"provider_message_id,omitempty"`
	LastError         *string    `json:"last_error,omitempty"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// LogResponse is a single notification log row.
type LogResponse struct {
	ID            int64           `json:"id"`
	DeliveryID    *int64          `json:"delivery_id,omitempty"`
	Channel       *string         `json:"channel,omitempty"`
	Service       string          `json:"service"`
	Level         string          `json:"level"`
	Event         string          `json:"event"`
	Message       *string         `json:"message,omitempty"`
	CorrelationID *string         `json:"correlation_id,omitempty"`
	TraceID       *string         `json:"trace_id,omitempty"`
	Details       json.RawMessage `json:"details,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// FromNotification maps a DTO to its response.
func FromNotification(n dto.Notification, stats *dto.DeliveryStats) NotificationResponse {
	return NotificationResponse{
		ID:                n.ID,
		BatchID:           n.BatchID,
		IdempotencyKey:    n.IdempotencyKey,
		CorrelationID:     n.CorrelationID,
		TraceID:           tracing.TraceIDFromCarrier(n.TraceContext),
		Channel:           n.Channel,
		Priority:          n.Priority,
		Status:            n.Status,
		AudienceType:      n.AudienceType,
		AudienceGroupID:   n.AudienceGroupID,
		ContentID:         n.ContentID,
		Content:           n.Content,
		CustomizedContent: n.CustomizedContent,
		ScheduledAt:       n.ScheduledAt,
		TotalRecipients:   n.TotalRecipients,
		ErrorMessage:      n.ErrorMessage,
		Stats:             stats,
		ProcessingStarted: n.ProcessingStarted,
		SendingStarted:    n.SendingStarted,
		CompletedAt:       n.CompletedAt,
		CancelledAt:       n.CancelledAt,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
	}
}

// FromDelivery maps a delivery DTO.
func FromDelivery(d dto.Delivery) DeliveryResponse {
	return DeliveryResponse{
		ID: d.ID, ContactID: d.ContactID, Recipient: d.Recipient, BatchNo: d.BatchNo, Status: d.Status,
		RetryCount: d.RetryCount, NextRetryAt: d.NextRetryAt, ProviderMessageID: d.ProviderMessageID,
		LastError: d.LastError, SentAt: d.SentAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// FromLog maps a log DTO.
func FromLog(l dto.NotificationLog) LogResponse {
	return LogResponse{
		ID: l.ID, DeliveryID: l.DeliveryID, Channel: l.Channel, Service: l.Service, Level: l.Level,
		Event: l.Event, Message: l.Message, CorrelationID: l.CorrelationID, TraceID: l.TraceID, Details: l.Details, CreatedAt: l.CreatedAt,
	}
}
