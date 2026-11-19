// Package config loads the Push sender configuration.
package config

import (
	"time"

	"notifier/pkg/config"
	"notifier/pkg/postgres"
	"notifier/pkg/rabbitmq"
)

// Channel handled by this sender.
const Channel = "push"

// Config holds every sender setting.
type Config struct {
	ServiceName string
	LogLevel    string
	HealthPort  string

	DatabaseURL string
	RabbitURL   string

	OTELEnabled  bool
	OTELEndpoint string

	ProviderURL     string
	ProviderTimeout time.Duration

	Workers         int           // concurrent queue messages per instance
	Prefetch        int           // unacked messages per instance
	SendConcurrency int           // concurrent provider calls per message
	RateLimit       int           // messages / second / channel (global, all replicas)
	MaxRetries      int           // attempts before a delivery becomes ERROR
	RetryDelay      time.Duration // delay before a failed delivery is retried
	LeaseDuration   time.Duration // how long a claimed delivery is locked
	LogEachDelivery bool          // write one notification_logs row per delivery
}

// Load reads configuration from environment variables.
func Load() Config {
	host := config.String("POSTGRES_HOST", "localhost")
	port := config.String("POSTGRES_PORT", "5432")
	db := config.String("POSTGRES_DB", "notifications")
	ssl := config.String("POSTGRES_SSLMODE", "disable")

	return Config{
		ServiceName: "pushsender",
		LogLevel:    config.String("LOG_LEVEL", "info"),
		HealthPort:  config.String("PUSH_SENDER_HEALTH_PORT", "8085"),

		DatabaseURL: postgres.DSN(host, port,
			config.String("POSTGRES_USER", "notifier"), config.String("POSTGRES_PASSWORD", ""), db, ssl),
		RabbitURL: rabbitmq.URL(
			config.String("RABBITMQ_USER", "guest"), config.String("RABBITMQ_PASSWORD", "guest"),
			config.String("RABBITMQ_HOST", "localhost"), config.String("RABBITMQ_PORT", "5672"),
			config.String("RABBITMQ_VHOST", "/")),

		OTELEnabled:  config.Bool("OTEL_ENABLED", false),
		OTELEndpoint: config.String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),

		ProviderURL:     config.String("PUSH_PROVIDER_URL", config.String("PROVIDER_URL", "")),
		ProviderTimeout: config.Duration("PROVIDER_TIMEOUT", 5*time.Second),

		Workers:         config.Int("SENDER_WORKERS", 2),
		Prefetch:        config.Int("SENDER_PREFETCH", 2),
		SendConcurrency: config.Int("SENDER_SEND_CONCURRENCY", 20),
		RateLimit:       config.Int("SENDER_RATE_LIMIT_PER_SECOND", 100),
		MaxRetries:      config.Int("SENDER_MAX_RETRIES", 3),
		RetryDelay:      config.Duration("SENDER_RETRY_DELAY", 10*time.Minute),
		LeaseDuration:   config.Duration("SENDER_LEASE_DURATION", 2*time.Minute),
		LogEachDelivery: config.Bool("SENDER_LOG_EACH_DELIVERY", true),
	}
}
