// Package healthserver exposes /health, /ready and /metrics for background
// worker services that do not have their own HTTP API.
package healthserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Check is a readiness probe for a dependency.
type Check func(ctx context.Context) error

// LivenessHandler always answers 200 while the process is running.
func LivenessHandler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": service, "time": time.Now().UTC()})
	}
}

// ReadinessHandler runs every dependency check and returns 503 if one fails.
func ReadinessHandler(service string, checks map[string]Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		results := make(map[string]string, len(checks))
		status := http.StatusOK
		for name, check := range checks {
			if err := check(ctx); err != nil {
				results[name] = err.Error()
				status = http.StatusServiceUnavailable
			} else {
				results[name] = "ok"
			}
		}
		overall := "ok"
		if status != http.StatusOK {
			overall = "degraded"
		}
		writeJSON(w, status, map[string]any{"status": overall, "service": service, "checks": results})
	}
}

// Run starts an HTTP server with health and metrics endpoints until ctx ends.
func Run(ctx context.Context, addr, service string, checks map[string]Check, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("GET /health", LivenessHandler(service))
	mux.Handle("GET /ready", ReadinessHandler(service, checks))
	mux.Handle("GET /metrics", promhttp.Handler())

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Info("health server listening", slog.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("health server failed", slog.Any("error", err))
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
