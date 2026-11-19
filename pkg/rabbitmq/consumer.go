package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrRequeue can be returned (wrapped) by a handler to ask for the message to
// be put back on the queue instead of being dead-lettered.
var ErrRequeue = errors.New("requeue message")

// Handler processes a single delivery. Returning nil acks the message,
// ErrRequeue nacks with requeue, any other error dead-letters it.
type Handler func(ctx context.Context, d amqp.Delivery) error

// ConsumerConfig configures a Consumer.
type ConsumerConfig struct {
	Queue          string
	Prefetch       int
	Workers        int
	HandlerTimeout time.Duration
}

// Consumer is a competing consumer: N instances (and N workers inside each
// instance) read from one queue and every message is delivered to only one
// of them. Manual acks + prefetch make sure unacked work is redelivered to
// another consumer if an instance dies.
type Consumer struct {
	client  *Client
	cfg     ConsumerConfig
	handler Handler
	log     *slog.Logger
}

// NewConsumer builds a consumer.
func NewConsumer(client *Client, cfg ConsumerConfig, handler Handler, log *slog.Logger) *Consumer {
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 1
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.HandlerTimeout <= 0 {
		cfg.HandlerTimeout = 5 * time.Minute
	}
	return &Consumer{client: client, cfg: cfg, handler: handler, log: log}
}

// Run consumes until ctx is cancelled, reconnecting on failures.
func (c *Consumer) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		err := c.consume(ctx)
		if ctx.Err() != nil {
			return nil
		}
		c.log.Error("consumer stopped, reconnecting", slog.String("queue", c.cfg.Queue), slog.Any("error", err))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 15*time.Second {
			backoff *= 2
		}
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	ch, err := c.client.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := DeclareTopology(ch); err != nil {
		return err
	}
	if err := ch.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("qos: %w", err)
	}
	host, _ := os.Hostname()
	tag := fmt.Sprintf("%s-%s-%d", c.cfg.Queue, host, time.Now().UnixNano())
	deliveries, err := ch.Consume(c.cfg.Queue, tag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}
	c.log.Info("consuming", slog.String("queue", c.cfg.Queue), slog.Int("workers", c.cfg.Workers), slog.Int("prefetch", c.cfg.Prefetch))

	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range deliveries {
				c.dispatch(d)
			}
		}()
	}

	closed := ch.NotifyClose(make(chan *amqp.Error, 1))
	select {
	case <-ctx.Done():
		// Stop receiving new deliveries; in-flight ones finish and get acked.
		_ = ch.Cancel(tag, false)
		wg.Wait()
		return nil
	case amqpErr := <-closed:
		wg.Wait()
		if amqpErr != nil {
			return amqpErr
		}
		return errors.New("channel closed")
	}
}

func (c *Consumer) dispatch(d amqp.Delivery) {
	// The handler must not be interrupted by shutdown mid-way, so it gets its
	// own timeout-bound context rather than the consumer's.
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.HandlerTimeout)
	defer cancel()
	ctx = ExtractHeaders(ctx, d.Headers)

	err := c.safeHandle(ctx, d)
	switch {
	case err == nil:
		if ackErr := d.Ack(false); ackErr != nil {
			c.log.ErrorContext(ctx, "ack failed", slog.Any("error", ackErr))
		}
	case errors.Is(err, ErrRequeue):
		c.log.WarnContext(ctx, "message requeued", slog.String("queue", c.cfg.Queue), slog.Any("error", err))
		time.Sleep(time.Second)
		_ = d.Nack(false, true)
	default:
		c.log.ErrorContext(ctx, "message dead-lettered", slog.String("queue", c.cfg.Queue), slog.Any("error", err))
		_ = d.Nack(false, false)
	}
}

func (c *Consumer) safeHandle(ctx context.Context, d amqp.Delivery) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return c.handler(ctx, d)
}
