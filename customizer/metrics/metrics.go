// Package metrics declares the Customizer's Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RecipientsCustomized counts personalised recipients by result.
	RecipientsCustomized = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "customizer_recipients_total",
		Help: "Recipients processed by the customizer by result.",
	}, []string{"channel", "result"})

	// BatchDuration observes the time to personalise one batch.
	BatchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "customizer_batch_duration_seconds",
		Help:    "Time to personalise and forward one batch.",
		Buckets: prometheus.DefBuckets,
	}, []string{"channel"})
)
