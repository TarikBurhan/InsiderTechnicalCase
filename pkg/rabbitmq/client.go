// Package rabbitmq wraps amqp091-go with reconnect handling, a confirmed
// publisher and a competing-consumer worker pool.
package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// URL builds an amqp url from its parts.
func URL(user, password, host, port, vhost string) string {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/" + url.PathEscape(trimSlash(vhost)),
	}
	return u.String()
}

func trimSlash(v string) string {
	if v == "/" {
		return ""
	}
	return v
}

// Client owns a single AMQP connection and re-dials it lazily when closed.
type Client struct {
	url string
	log *slog.Logger

	mu   sync.Mutex
	conn *amqp.Connection
}

// NewClient dials RabbitMQ, retrying until ctx is cancelled.
func NewClient(ctx context.Context, amqpURL string, log *slog.Logger) (*Client, error) {
	c := &Client{url: amqpURL, log: log}
	backoff := 500 * time.Millisecond
	for attempt := 1; ; attempt++ {
		if _, err := c.connection(); err == nil {
			return c, nil
		} else if attempt >= 30 {
			return nil, fmt.Errorf("rabbitmq not reachable: %w", err)
		} else {
			log.Warn("rabbitmq not ready, retrying", slog.Int("attempt", attempt), slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) connection() (*amqp.Connection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn, nil
	}
	conn, err := amqp.DialConfig(c.url, amqp.Config{Heartbeat: 10 * time.Second, Locale: "en_US"})
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

// Channel opens a new channel on the (possibly re-dialed) connection.
func (c *Client) Channel() (*amqp.Channel, error) {
	conn, err := c.connection()
	if err != nil {
		return nil, err
	}
	return conn.Channel()
}

// Healthy reports whether the underlying connection is open.
func (c *Client) Healthy(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.conn.IsClosed() {
		return errors.New("rabbitmq connection closed")
	}
	return nil
}

// QueueDepth returns the number of ready messages and consumers of a queue.
func (c *Client) QueueDepth(name string) (messages int, consumers int, err error) {
	ch, err := c.Channel()
	if err != nil {
		return 0, 0, err
	}
	defer ch.Close()
	q, err := ch.QueueDeclarePassive(name, true, false, false, false, nil)
	if err != nil {
		return 0, 0, err
	}
	return q.Messages, q.Consumers, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}
