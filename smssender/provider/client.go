// Package provider calls the external SMS provider (webhook.site).
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"notifier/pkg/correlation"
	"notifier/smssender/request"
	"notifier/smssender/response"
)

// SendError describes a failed provider call.
type SendError struct {
	StatusCode int
	Retryable  bool
	Message    string
}

func (e *SendError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("provider returned %d: %s", e.StatusCode, e.Message)
	}
	return "provider call failed: " + e.Message
}

// IsPermanent reports whether retrying err makes no sense.
func IsPermanent(err error) bool {
	var se *SendError
	return errors.As(err, &se) && !se.Retryable
}

// Client is an HTTP client for the provider.
type Client struct {
	url  string
	http *http.Client
}

// NewClient creates a provider client.
func NewClient(url string, timeout time.Duration) *Client {
	return &Client{
		url: url,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Send posts one message. idempotencyKey lets the provider de-duplicate
// retries of the same delivery attempt.
func (c *Client) Send(ctx context.Context, req request.ProviderRequest, idempotencyKey string) (response.ProviderResponse, error) {
	var out response.ProviderResponse
	body, err := json.Marshal(req)
	if err != nil {
		return out, &SendError{Message: err.Error()}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return out, &SendError{Message: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotency-Key", idempotencyKey)
	if id := correlation.FromContext(ctx); id != "" {
		httpReq.Header.Set(correlation.HeaderName, id)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return out, &SendError{Retryable: true, Message: err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, &SendError{
			StatusCode: resp.StatusCode,
			Retryable:  isRetryableStatus(resp.StatusCode),
			Message:    truncate(string(raw), 300),
		}
	}
	// Expected: 202 {"messageId": "...", "status": "accepted", "timestamp": "..."}.
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

// isRetryableStatus: server errors, throttling and timeouts are retried;
// other 4xx mean the request itself is wrong and will never succeed.
func isRetryableStatus(code int) bool {
	return code >= 500 || code == http.StatusTooManyRequests || code == http.StatusRequestTimeout
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
