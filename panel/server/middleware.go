package server

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"notifier/panel/metrics"
	"notifier/pkg/correlation"
)

// statusRecorder captures the response status while still supporting
// WebSocket hijacking.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacking not supported")
	}
	r.status = http.StatusSwitchingProtocols
	return h.Hijack()
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Correlation reads X-Correlation-ID (or generates one), stores it in the
// request context and echoes it in the response.
func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(correlation.HeaderName)
		if id == "" || len(id) > 128 {
			id = correlation.NewID()
		}
		w.Header().Set(correlation.HeaderName, id)
		next.ServeHTTP(w, r.WithContext(correlation.WithID(r.Context(), id)))
	})
}

// Observe creates a tracing span, records Prometheus metrics and writes a
// structured access log line per request.
func Observe(log *slog.Logger) func(http.Handler) http.Handler {
	tracer := otel.Tracer("panel/http")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			req := r.WithContext(ctx)
			next.ServeHTTP(rec, req)

			route := req.Pattern // set by ServeMux on the request it served
			if route == "" {
				route = "unmatched"
			}
			duration := time.Since(start)
			metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
			metrics.HTTPDuration.WithLabelValues(route, r.Method).Observe(duration.Seconds())
			if req.Pattern != "" {
				span.SetName(req.Pattern)
			}
			span.SetAttributes(attribute.String("http.route", route), attribute.Int("http.status_code", rec.status))
			if rec.status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(rec.status))
			}
			log.InfoContext(ctx, "http request",
				slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("route", route),
				slog.Int("status", rec.status), slog.Int64("duration_ms", duration.Milliseconds()),
				slog.String("remote", r.RemoteAddr))
		})
	}
}

// Recover turns panics into 500 responses.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rv := recover(); rv != nil {
					log.ErrorContext(r.Context(), "panic recovered", slog.Any("panic", rv))
					http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
