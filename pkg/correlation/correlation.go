// Package correlation carries a correlation id through contexts, HTTP headers
// and queue messages so that a single notification can be followed across
// every service in the logs.
package correlation

import (
	"context"

	"github.com/google/uuid"
)

// HeaderName is used both for HTTP headers and AMQP message headers.
const HeaderName = "X-Correlation-ID"

type ctxKey struct{}

// NewID generates a new correlation id.
func NewID() string { return uuid.NewString() }

// WithID stores the correlation id in ctx.
func WithID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the correlation id stored in ctx (or "").
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
