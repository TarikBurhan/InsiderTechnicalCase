// Package metrics declares the Manager's Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// NotificationsProcessed counts processed notifications by outcome.
	NotificationsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "manager_notifications_processed_total",
		Help: "Notifications processed by the manager by result.",
	}, []string{"channel", "result"})

	// ProcessingDuration observes how long producing a notification took.
	ProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "manager_processing_duration_seconds",
		Help:    "Time spent resolving the audience and producing all batches.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
	}, []string{"channel"})

	// BatchesPublished counts produced queue messages.
	BatchesPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "manager_batches_published_total",
		Help: "Batches (queue messages) published by queue.",
	}, []string{"queue"})

	// RecipientsProduced counts produced recipients.
	RecipientsProduced = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "manager_recipients_produced_total",
		Help: "Recipients produced to queues by channel.",
	}, []string{"channel"})

	// RetriesScheduled counts deliveries re-produced for retry.
	RetriesScheduled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "manager_retries_scheduled_total",
		Help: "Deliveries re-produced for a retry attempt.",
	}, []string{"channel"})

	// StaleSwept counts deliveries stuck in SENDING that were recovered.
	StaleSwept = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "manager_stale_deliveries_swept_total",
		Help: "Deliveries stuck in SENDING moved back to RETRY.",
	}, []string{"channel"})

	// InFlight is the number of notifications currently being processed.
	InFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "manager_notifications_in_flight",
		Help: "Notifications currently being processed by this instance.",
	})
)
