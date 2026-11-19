// Package publisher builds queue messages and produces them to RabbitMQ.
package publisher

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"notifier/manager/dto"
	"notifier/manager/metrics"
	"notifier/pkg/rabbitmq"
)

// BatchPublisher produces recipient batches to the right queue.
type BatchPublisher struct {
	publisher *rabbitmq.Publisher
}

// New creates a batch publisher.
func New(p *rabbitmq.Publisher) *BatchPublisher { return &BatchPublisher{publisher: p} }

// Batch describes one queue message to publish.
type Batch struct {
	Notification dto.Notification
	Content      dto.MessageContent
	BatchNo      int
	Attempt      int
	Targets      []dto.DeliveryTarget
}

// TargetQueue returns the queue a notification's batches go to: the
// Customizer when content must be personalised, else the channel's sender.
func TargetQueue(n dto.Notification) (string, error) {
	if n.CustomizedContent {
		return rabbitmq.QueueCustomizer, nil
	}
	return rabbitmq.QueueForChannel(n.Channel)
}

// BuildMessage converts a batch into the queue message contract.
func BuildMessage(b Batch) dto.QueueMessage {
	recipients := make([]dto.QueueRecipient, len(b.Targets))
	for i, t := range b.Targets {
		recipients[i] = dto.QueueRecipient{DeliveryID: t.DeliveryID, ContactID: t.ContactID, To: t.Recipient, Attempt: t.Attempt}
	}
	return dto.QueueMessage{
		MessageID:      uuid.NewString(),
		NotificationID: b.Notification.ID,
		Channel:        b.Notification.Channel,
		Priority:       b.Notification.Priority,
		CorrelationID:  b.Notification.CorrelationID,
		BatchNo:        b.BatchNo,
		Attempt:        b.Attempt,
		Customized:     b.Notification.CustomizedContent,
		Content:        b.Content,
		Recipients:     recipients,
		CreatedAt:      time.Now().UTC(),
	}
}

// Publish produces one batch with the notification's priority.
func (p *BatchPublisher) Publish(ctx context.Context, b Batch) (string, error) {
	queue, err := TargetQueue(b.Notification)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(BuildMessage(b))
	if err != nil {
		return "", err
	}
	if err := p.publisher.Publish(ctx, queue, body, rabbitmq.PriorityValue(b.Notification.Priority)); err != nil {
		return queue, err
	}
	metrics.BatchesPublished.WithLabelValues(queue).Inc()
	metrics.RecipientsProduced.WithLabelValues(b.Notification.Channel).Add(float64(len(b.Targets)))
	return queue, nil
}

// Chunk splits targets into slices of at most size elements.
func Chunk[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = 1
	}
	var out [][]T
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[start:end])
	}
	return out
}
