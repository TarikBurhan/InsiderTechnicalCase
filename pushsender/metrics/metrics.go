// Package metrics declares the Push sender's Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Sends counts provider calls by result (sent / retry / error).
	Sends = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pushsender_deliveries_total",
		Help: "Push deliveries by result.",
	}, []string{"result"})

	// ProviderLatency observes provider call latency.
	ProviderLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pushsender_provider_latency_seconds",
		Help:    "Latency of Push provider calls.",
		Buckets: prometheus.DefBuckets,
	})

	// Batches counts consumed queue messages by outcome.
	Batches = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pushsender_batches_total",
		Help: "Consumed Push batches by outcome.",
	}, []string{"outcome"})
)
