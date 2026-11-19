package tracing

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const maxStatementLength = 1000

type pgxSpanKey struct{}

// PgxTracer creates a span for every SQL statement, batch and COPY executed
// inside a traced context. It is installed on every pgx pool.
type PgxTracer struct{}

// NewPgxTracer creates the tracer.
func NewPgxTracer() *PgxTracer { return &PgxTracer{} }

func (t *PgxTracer) start(ctx context.Context, name, statement string) context.Context {
	ctx, span := StartChild(ctx, name, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system", "postgresql")))
	if !span.SpanContext().IsValid() {
		return ctx
	}
	if statement != "" {
		span.SetAttributes(attribute.String("db.statement", truncate(strings.Join(strings.Fields(statement), " "), maxStatementLength)))
	}
	return context.WithValue(ctx, pgxSpanKey{}, span)
}

func (t *PgxTracer) end(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span, ok := ctx.Value(pgxSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	span.SetAttributes(attrs...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// TraceQueryStart implements pgx.QueryTracer.
func (t *PgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return t.start(ctx, "db "+operation(data.SQL), data.SQL)
}

// TraceQueryEnd implements pgx.QueryTracer.
func (t *PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	t.end(ctx, data.Err, attribute.Int64("db.rows_affected", data.CommandTag.RowsAffected()))
}

// TraceBatchStart implements pgx.BatchTracer.
func (t *PgxTracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchStartData) context.Context {
	ctx = t.start(ctx, "db batch", "")
	if span, ok := ctx.Value(pgxSpanKey{}).(trace.Span); ok && data.Batch != nil {
		span.SetAttributes(attribute.Int("db.batch.size", data.Batch.Len()))
	}
	return ctx
}

// TraceBatchQuery implements pgx.BatchTracer.
func (t *PgxTracer) TraceBatchQuery(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchQueryData) {
	if span, ok := ctx.Value(pgxSpanKey{}).(trace.Span); ok {
		span.AddEvent("query", trace.WithAttributes(attribute.String("db.statement", truncate(data.SQL, 200))))
		if data.Err != nil {
			span.RecordError(data.Err)
		}
	}
}

// TraceBatchEnd implements pgx.BatchTracer.
func (t *PgxTracer) TraceBatchEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchEndData) {
	t.end(ctx, data.Err)
}

// TraceCopyFromStart implements pgx.CopyFromTracer.
func (t *PgxTracer) TraceCopyFromStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceCopyFromStartData) context.Context {
	return t.start(ctx, "db COPY "+data.TableName.Sanitize(), "")
}

// TraceCopyFromEnd implements pgx.CopyFromTracer.
func (t *PgxTracer) TraceCopyFromEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceCopyFromEndData) {
	t.end(ctx, data.Err, attribute.Int64("db.rows_affected", data.CommandTag.RowsAffected()))
}

// operation returns the first SQL keyword (SELECT, INSERT, UPDATE, WITH ...).
func operation(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "query"
	}
	return strings.ToUpper(fields[0])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
