package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes persistent messages with publisher confirms, so a
// message is only considered produced once the broker has stored it.
type Publisher struct {
	client *Client

	mu sync.Mutex
	ch *amqp.Channel
}

// NewPublisher creates a publisher on top of client.
func NewPublisher(client *Client) *Publisher { return &Publisher{client: client} }

func (p *Publisher) channel() (*amqp.Channel, error) {
	if p.ch != nil && !p.ch.IsClosed() {
		return p.ch, nil
	}
	ch, err := p.client.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("enable confirms: %w", err)
	}
	p.ch = ch
	return ch, nil
}

// Publish sends body to the notifications exchange with the given routing
// key and priority. It retries a few times on channel/connection failures.
func (p *Publisher) Publish(ctx context.Context, routingKey string, body []byte, priority uint8) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		if lastErr = p.publishOnce(ctx, routingKey, body, priority); lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("publish to %s: %w", routingKey, lastErr)
}

func (p *Publisher) publishOnce(ctx context.Context, routingKey string, body []byte, priority uint8) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch, err := p.channel()
	if err != nil {
		return err
	}
	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Priority:     priority,
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Headers:      InjectHeaders(ctx, amqp.Table{}),
		Body:         body,
	}
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, ExchangeName, routingKey, true, false, msg)
	if err != nil {
		p.reset()
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	acked, err := confirm.WaitContext(waitCtx)
	if err != nil {
		p.reset()
		return err
	}
	if !acked {
		return errors.New("broker nacked message")
	}
	return nil
}

func (p *Publisher) reset() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	p.ch = nil
}

// Close closes the publishing channel.
func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reset()
}
