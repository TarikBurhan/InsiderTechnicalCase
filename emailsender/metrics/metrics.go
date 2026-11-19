// Package metrics declares the Email sender's Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Sends counts provider calls by result (sent / retry / error).
	Sends = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "emailsender_deliveries_total",
		Help: "Email deliveries by result.",
	}, []string{"result"})

	// ProviderLatency observes provider call latency.
	ProviderLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "emailsender_provider_latency_seconds",
		Help:    "Latency of Email provider calls.",
		Buckets: prometheus.DefBuckets,
	})

	// Batches counts consumed queue messages by outcome.
	Batches = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "emailsender_batches_total",
		Help: "Consumed Email batches by outcome.",
	}, []string{"outcome"})
)
