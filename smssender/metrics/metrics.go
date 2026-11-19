// Package metrics declares the SMS sender's Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Sends counts provider calls by result (sent / retry / error).
	Sends = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smssender_deliveries_total",
		Help: "SMS deliveries by result.",
	}, []string{"result"})

	// ProviderLatency observes provider call latency.
	ProviderLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "smssender_provider_latency_seconds",
		Help:    "Latency of SMS provider calls.",
		Buckets: prometheus.DefBuckets,
	})

	// Batches counts consumed queue messages by outcome.
	Batches = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smssender_batches_total",
		Help: "Consumed SMS batches by outcome.",
	}, []string{"outcome"})
)
