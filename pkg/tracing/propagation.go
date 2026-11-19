package tracing

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Carrier serialises the trace context of ctx (W3C traceparent/tracestate)
// so it can be stored in the database and continued later by another
// service, e.g. the Manager picking a notification up by polling.
func Carrier(ctx context.Context) string {
	m := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, m)
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// FromCarrier restores a trace context produced by Carrier into ctx. Spans
// started from the returned context join the original trace.
func FromCarrier(ctx context.Context, carrier string) context.Context {
	if carrier == "" {
		return ctx
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(carrier), &m); err != nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(m))
}

// TraceID returns the trace id of ctx or "".
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

// TraceIDFromCarrier returns the trace id stored in a carrier or "".
func TraceIDFromCarrier(carrier string) string {
	return TraceID(FromCarrier(context.Background(), carrier))
}

// StartChild starts a span only when ctx already belongs to a trace. Work
// that is not part of a traced request (polling loops, health checks) does
// not create orphan root spans.
func StartChild(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx, noop.Span{}
	}
	return otel.Tracer("notifier").Start(ctx, name, opts...)
}
