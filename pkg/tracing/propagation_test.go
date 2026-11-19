package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func setup(t *testing.T) trace.Tracer {
	t.Helper()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp.Tracer("test")
}

func TestCarrierRoundTrip(t *testing.T) {
	tracer := setup(t)
	ctx, span := tracer.Start(context.Background(), "panel request")
	defer span.End()

	carrier := Carrier(ctx)
	if carrier == "" {
		t.Fatal("carrier must not be empty inside a span")
	}
	restored := FromCarrier(context.Background(), carrier)
	if TraceID(restored) != span.SpanContext().TraceID().String() {
		t.Fatalf("trace id lost: %s vs %s", TraceID(restored), span.SpanContext().TraceID())
	}
	if TraceIDFromCarrier(carrier) != span.SpanContext().TraceID().String() {
		t.Fatal("TraceIDFromCarrier must return the original trace id")
	}

	// A span started from the restored context continues the same trace.
	_, child := tracer.Start(restored, "manager process")
	defer child.End()
	if child.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatal("child span must belong to the original trace")
	}
}

func TestCarrierEmptyOutsideTrace(t *testing.T) {
	setup(t)
	if Carrier(context.Background()) != "" {
		t.Fatal("no trace -> empty carrier")
	}
	if TraceID(FromCarrier(context.Background(), "not json")) != "" {
		t.Fatal("invalid carrier must be ignored")
	}
}

func TestStartChildSkipsRootSpans(t *testing.T) {
	tracer := setup(t)
	ctx, span := StartChild(context.Background(), "poll")
	if span.SpanContext().IsValid() || ctx != context.Background() {
		t.Fatal("StartChild must not create root spans")
	}
	parentCtx, parent := tracer.Start(context.Background(), "parent")
	defer parent.End()
	_, child := StartChild(parentCtx, "child")
	defer child.End()
	if child.SpanContext().TraceID() != parent.SpanContext().TraceID() {
		t.Fatal("child must join the parent trace")
	}
}

func TestOperation(t *testing.T) {
	if operation("  select 1") != "SELECT" || operation("") != "query" {
		t.Fatal("unexpected operation name")
	}
}
