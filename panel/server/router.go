// Package server wires HTTP routes and middleware for the Panel API.
package server

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"notifier/panel/handlers"
	"notifier/pkg/healthserver"
)

// Handlers groups every handler the router needs.
type Handlers struct {
	Notifications *handlers.NotificationHandler
	Audiences     *handlers.AudienceHandler
	Contents      *handlers.ContentHandler
	Metrics       *handlers.MetricsHandler
	WebSocket     *handlers.WebSocketHandler
	Settings      *handlers.SettingHandler
	ReadyChecks   map[string]healthserver.Check
}

// NewRouter builds the HTTP handler with all routes and middleware.
func NewRouter(h Handlers, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Notifications
	mux.HandleFunc("POST /api/v1/notifications", h.Notifications.Create)
	mux.HandleFunc("POST /api/v1/notifications/batch", h.Notifications.CreateBatch)
	mux.HandleFunc("GET /api/v1/notifications", h.Notifications.List)
	mux.HandleFunc("GET /api/v1/batches/{batchId}", h.Notifications.GetBatch)
	mux.HandleFunc("GET /api/v1/notifications/{id}", h.Notifications.Get)
	mux.HandleFunc("POST /api/v1/notifications/{id}/cancel", h.Notifications.Cancel)
	mux.HandleFunc("GET /api/v1/notifications/{id}/deliveries", h.Notifications.Deliveries)
	mux.HandleFunc("GET /api/v1/notifications/{id}/logs", h.Notifications.Logs)

	// Audiences
	mux.HandleFunc("POST /api/v1/audiences/validate", h.Audiences.Validate)
	mux.HandleFunc("POST /api/v1/audience-groups", h.Audiences.CreateGroup)
	mux.HandleFunc("GET /api/v1/audience-groups", h.Audiences.ListGroups)
	mux.HandleFunc("GET /api/v1/audience-groups/{id}", h.Audiences.GetGroup)
	mux.HandleFunc("DELETE /api/v1/audience-groups/{id}", h.Audiences.DeleteGroup)

	// Contents / templates
	mux.HandleFunc("POST /api/v1/contents/{channel}", h.Contents.Create)
	mux.HandleFunc("GET /api/v1/contents/{channel}", h.Contents.List)
	mux.HandleFunc("GET /api/v1/contents/{channel}/{id}", h.Contents.Get)
	mux.HandleFunc("DELETE /api/v1/contents/{channel}/{id}", h.Contents.Delete)

	// Account settings / time zone
	mux.HandleFunc("GET /api/v1/settings", h.Settings.List)
	mux.HandleFunc("GET /api/v1/settings/{key}", h.Settings.Get)
	mux.HandleFunc("PUT /api/v1/settings/{key}", h.Settings.Update)
	mux.HandleFunc("GET /api/v1/system/time", h.Settings.TimeStatus)

	// Observability
	mux.HandleFunc("GET /api/v1/metrics", h.Metrics.Get)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /health", healthserver.LivenessHandler("panel"))
	mux.Handle("GET /ready", healthserver.ReadinessHandler("panel", h.ReadyChecks))

	// Real-time updates
	mux.HandleFunc("GET /ws/notifications", h.WebSocket.Serve)

	// API documentation
	mux.HandleFunc("GET /swagger", handlers.SwaggerUI)
	mux.HandleFunc("GET /swagger/openapi.yaml", handlers.OpenAPISpec)

	var handler http.Handler = mux
	handler = Observe(log)(handler)
	handler = Correlation(handler)
	handler = Recover(log)(handler)
	return handler
}
