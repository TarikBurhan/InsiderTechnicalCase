package response

import "time"

// MetricsResponse is the real-time metrics snapshot.
type MetricsResponse struct {
	GeneratedAt   time.Time        `json:"generated_at"`
	WindowSeconds int              `json:"window_seconds"`
	Queues        []QueueMetric    `json:"queues"`
	Channels      []ChannelMetric  `json:"channels"`
	Notifications map[string]int64 `json:"notifications_by_status"`
	WebSocket     map[string]int   `json:"websocket"`
}

// QueueMetric is the depth of a queue.
type QueueMetric struct {
	Name      string `json:"name"`
	Depth     int    `json:"depth"`
	Consumers int    `json:"consumers"`
	Error     string `json:"error,omitempty"`
}

// ChannelMetric aggregates delivery outcomes for a channel.
type ChannelMetric struct {
	Channel      string  `json:"channel"`
	Sent         int64   `json:"sent"`
	Failed       int64   `json:"failed"`
	Retrying     int64   `json:"retrying"`
	InFlight     int64   `json:"in_flight"`
	SuccessRate  float64 `json:"success_rate"`
	FailureRate  float64 `json:"failure_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
}

// HealthResponse is returned by /health and /ready.
type HealthResponse struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Checks  map[string]string `json:"checks,omitempty"`
}
