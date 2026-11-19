package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"

	"notifier/pkg/correlation"
)

// headerCarrier adapts amqp.Table to the OpenTelemetry TextMapCarrier so that
// trace context travels with queue messages (distributed tracing).
type headerCarrier amqp.Table

func (h headerCarrier) Get(key string) string {
	if v, ok := h[key].(string); ok {
		return v
	}
	return ""
}

func (h headerCarrier) Set(key, value string) { h[key] = value }

func (h headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

// InjectHeaders writes correlation id and trace context into AMQP headers.
func InjectHeaders(ctx context.Context, headers amqp.Table) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}
	if id := correlation.FromContext(ctx); id != "" {
		headers[correlation.HeaderName] = id
	}
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier(headers))
	return headers
}

// ExtractHeaders restores correlation id and trace context from headers.
func ExtractHeaders(ctx context.Context, headers amqp.Table) context.Context {
	if headers == nil {
		return ctx
	}
	if id, ok := headers[correlation.HeaderName].(string); ok {
		ctx = correlation.WithID(ctx, id)
	}
	return otel.GetTextMapPropagator().Extract(ctx, headerCarrier(headers))
}
