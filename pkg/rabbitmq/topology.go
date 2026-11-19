package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchange / queue names shared by every service. Each queue is a classic
// durable priority queue with a dead-letter queue attached. Multiple
// instances of a service consume the SAME queue, which makes them competing
// consumers: RabbitMQ hands every message to exactly one of them.
const (
	ExchangeName    = "notifications"
	DLXExchangeName = "notifications.dlx"

	QueueSMS        = "notifications.sms"
	QueueEmail      = "notifications.email"
	QueuePush       = "notifications.push"
	QueueCustomizer = "notifications.customizer"

	MaxPriority = 10
)

// AllQueues lists every work queue.
var AllQueues = []string{QueueSMS, QueueEmail, QueuePush, QueueCustomizer}

// QueueForChannel maps a notification channel to its sender queue.
func QueueForChannel(channel string) (string, error) {
	switch channel {
	case "sms":
		return QueueSMS, nil
	case "email":
		return QueueEmail, nil
	case "push":
		return QueuePush, nil
	default:
		return "", fmt.Errorf("unknown channel %q", channel)
	}
}

// DLQName returns the dead-letter queue name for a work queue.
func DLQName(queue string) string { return queue + ".dlq" }

// DeclareTopology idempotently declares exchanges, queues, DLQs and bindings.
func DeclareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeName, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(DLXExchangeName, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx: %w", err)
	}
	for _, q := range AllQueues {
		dlq := DLQName(q)
		if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare %s: %w", dlq, err)
		}
		if err := ch.QueueBind(dlq, dlq, DLXExchangeName, false, nil); err != nil {
			return fmt.Errorf("bind %s: %w", dlq, err)
		}
		args := amqp.Table{
			"x-max-priority":            int32(MaxPriority),
			"x-dead-letter-exchange":    DLXExchangeName,
			"x-dead-letter-routing-key": dlq,
		}
		if _, err := ch.QueueDeclare(q, true, false, false, false, args); err != nil {
			return fmt.Errorf("declare %s: %w", q, err)
		}
		if err := ch.QueueBind(q, q, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("bind %s: %w", q, err)
		}
	}
	return nil
}

// SetupTopology opens a channel on the client and declares the topology.
func SetupTopology(c *Client) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	return DeclareTopology(ch)
}

// PriorityValue converts a notification priority to an AMQP priority.
func PriorityValue(priority string) uint8 {
	switch priority {
	case "high":
		return 9
	case "low":
		return 1
	default:
		return 5
	}
}
