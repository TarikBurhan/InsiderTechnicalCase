// Package config loads the Manager service configuration.
package config

import (
	"time"

	"notifier/pkg/config"
	"notifier/pkg/postgres"
	"notifier/pkg/rabbitmq"
)

// Config holds every Manager setting.
type Config struct {
	ServiceName string
	LogLevel    string
	HealthPort  string

	DatabaseURL         string
	AudienceDatabaseURL string
	RabbitURL           string

	OTELEnabled  bool
	OTELEndpoint string

	// Polling of NEW notifications
	PollInterval time.Duration
	Workers      int
	BatchSize    int // recipients per queue message (rate limiter friendly, max 100)

	// Crash recovery of PROCESSING notifications
	HeartbeatInterval     time.Duration
	StaleProcessingAfter  time.Duration
	RecoveryInterval      time.Duration
	MaxProcessingAttempts int
	AudienceQueryTimeout  time.Duration

	// Retries / completion / stuck deliveries
	RetryPollInterval time.Duration
	RetryClaimLimit   int
	FinalizerInterval time.Duration
	SweeperInterval   time.Duration
	StaleSendingAfter time.Duration
	SweeperClaimLimit int
}

// Load reads configuration from environment variables.
func Load() Config {
	host := config.String("POSTGRES_HOST", "localhost")
	port := config.String("POSTGRES_PORT", "5432")
	db := config.String("POSTGRES_DB", "notifications")
	ssl := config.String("POSTGRES_SSLMODE", "disable")

	batch := config.Int("MANAGER_BATCH_SIZE", 100)
	if batch <= 0 || batch > 100 {
		batch = 100
	}

	return Config{
		ServiceName: "manager",
		LogLevel:    config.String("LOG_LEVEL", "info"),
		HealthPort:  config.String("MANAGER_HEALTH_PORT", "8081"),

		DatabaseURL: postgres.DSN(host, port,
			config.String("POSTGRES_USER", "notifier"), config.String("POSTGRES_PASSWORD", ""), db, ssl),
		AudienceDatabaseURL: postgres.DSN(host, port,
			config.String("AUDIENCE_DB_USER", "audience_reader"), config.String("AUDIENCE_DB_PASSWORD", ""), db, ssl),
		RabbitURL: rabbitmq.URL(
			config.String("RABBITMQ_USER", "guest"), config.String("RABBITMQ_PASSWORD", "guest"),
			config.String("RABBITMQ_HOST", "localhost"), config.String("RABBITMQ_PORT", "5672"),
			config.String("RABBITMQ_VHOST", "/")),

		OTELEnabled:  config.Bool("OTEL_ENABLED", false),
		OTELEndpoint: config.String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),

		PollInterval: config.Duration("MANAGER_POLL_INTERVAL", 2*time.Second),
		Workers:      config.Int("MANAGER_WORKERS", 4),
		BatchSize:    batch,

		HeartbeatInterval:     config.Duration("MANAGER_HEARTBEAT_INTERVAL", 10*time.Second),
		StaleProcessingAfter:  config.Duration("MANAGER_STALE_PROCESSING_AFTER", 30*time.Second),
		RecoveryInterval:      config.Duration("MANAGER_RECOVERY_INTERVAL", 15*time.Second),
		MaxProcessingAttempts: config.Int("MANAGER_MAX_PROCESSING_ATTEMPTS", 5),
		AudienceQueryTimeout:  config.Duration("MANAGER_AUDIENCE_QUERY_TIMEOUT", 30*time.Minute),

		RetryPollInterval: config.Duration("MANAGER_RETRY_POLL_INTERVAL", 10*time.Second),
		RetryClaimLimit:   config.Int("MANAGER_RETRY_CLAIM_LIMIT", 1000),
		FinalizerInterval: config.Duration("MANAGER_FINALIZER_INTERVAL", 10*time.Second),
		SweeperInterval:   config.Duration("MANAGER_SWEEPER_INTERVAL", time.Minute),
		StaleSendingAfter: config.Duration("MANAGER_STALE_SENDING_AFTER", 30*time.Minute),
		SweeperClaimLimit: config.Int("MANAGER_SWEEPER_CLAIM_LIMIT", 1000),
	}
}
