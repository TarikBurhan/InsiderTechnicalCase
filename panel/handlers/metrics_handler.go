package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"notifier/panel/response"
	"notifier/panel/service"
)

// ClientCounter reports connected WebSocket clients.
type ClientCounter interface {
	Count() int
}

// MetricsHandler serves the real-time JSON metrics endpoint.
type MetricsHandler struct {
	svc *service.MetricsService
	ws  ClientCounter
	log *slog.Logger
}

// NewMetricsHandler creates the handler.
func NewMetricsHandler(svc *service.MetricsService, ws ClientCounter, log *slog.Logger) *MetricsHandler {
	return &MetricsHandler{svc: svc, ws: ws, log: log}
}

// Get handles GET /api/v1/metrics?window=300 (seconds).
func (h *MetricsHandler) Get(w http.ResponseWriter, r *http.Request) {
	window, _ := strconv.Atoi(r.URL.Query().Get("window"))
	if window <= 0 || window > 86400 {
		window = 300
	}
	snap, err := h.svc.Snapshot(r.Context(), window)
	if err != nil {
		handleServiceError(w, r, h.log, err)
		return
	}

	out := response.MetricsResponse{
		GeneratedAt:   snap.GeneratedAt,
		WindowSeconds: snap.WindowSeconds,
		Notifications: map[string]int64{},
		WebSocket:     map[string]int{"connected_clients": h.ws.Count()},
	}
	for _, q := range snap.Queues {
		out.Queues = append(out.Queues, response.QueueMetric{Name: q.Name, Depth: q.Messages, Consumers: q.Consumers, Error: q.Error})
	}
	for _, c := range snap.Channels {
		m := response.ChannelMetric{
			Channel: c.Channel, Sent: c.Sent, Failed: c.Failed, Retrying: c.Retrying, InFlight: c.InFlight,
			AvgLatencyMs: c.AvgLatencyMs, P95LatencyMs: c.P95LatencyMs,
		}
		if done := c.Sent + c.Failed; done > 0 {
			m.SuccessRate = float64(c.Sent) / float64(done)
			m.FailureRate = float64(c.Failed) / float64(done)
		}
		out.Channels = append(out.Channels, m)
	}
	for _, sc := range snap.StatusCounts {
		out.Notifications[sc.Status] = sc.Count
	}
	writeJSON(w, http.StatusOK, out)
}
