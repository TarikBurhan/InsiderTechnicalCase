// Package metrics declares the Panel's Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequests counts HTTP requests by route, method and status.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "panel_http_requests_total",
		Help: "HTTP requests handled by the panel.",
	}, []string{"route", "method", "status"})

	// HTTPDuration observes HTTP request latency.
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "panel_http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method"})

	// RateLimited counts requests rejected with 429 by tier (read / write).
	RateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "panel_http_rate_limited_total",
		Help: "Requests rejected by the API rate limiter.",
	}, []string{"tier"})

	// AudienceCalculations counts background audience calculations by result.
	AudienceCalculations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "panel_audience_calculations_total",
		Help: "Background audience calculations by result (completed, empty, rejected, timeout, retry).",
	}, []string{"result"})
)
