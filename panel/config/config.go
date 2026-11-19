// Package config loads the Panel service configuration from the environment.
package config

import (
	"time"

	"notifier/pkg/config"
	"notifier/pkg/postgres"
	"notifier/pkg/rabbitmq"
)

// Config holds every setting the Panel API needs.
type Config struct {
	ServiceName string
	HTTPPort    string
	LogLevel    string

	DatabaseURL         string
	AudienceDatabaseURL string
	RabbitURL           string

	OTELEnabled  bool
	OTELEndpoint string

	MaxBatchSize              int
	MaxDirectRecipients       int
	AudienceValidationTimeout time.Duration
	AudienceExtraTables       []string
	DefaultPageSize           int
	MaxPageSize               int

	// API rate limiting (per client IP, per tier)
	RateLimitEnabled    bool
	RateLimitReadRPS    float64
	RateLimitReadBurst  int
	RateLimitWriteRPS   float64
	RateLimitWriteBurst int
	RateLimitTrustProxy bool
}

// Load reads configuration from environment variables.
func Load() Config {
	host := config.String("POSTGRES_HOST", "localhost")
	port := config.String("POSTGRES_PORT", "5432")
	db := config.String("POSTGRES_DB", "notifications")
	ssl := config.String("POSTGRES_SSLMODE", "disable")

	return Config{
		ServiceName: "panel",
		HTTPPort:    config.String("PANEL_PORT", "8080"),
		LogLevel:    config.String("LOG_LEVEL", "info"),

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

		MaxBatchSize:              config.Int("PANEL_MAX_BATCH_SIZE", 1000),
		MaxDirectRecipients:       config.Int("PANEL_MAX_DIRECT_RECIPIENTS", 10000),
		AudienceValidationTimeout: config.Duration("PANEL_AUDIENCE_VALIDATION_TIMEOUT", 10*time.Second),
		AudienceExtraTables:       config.List("PANEL_AUDIENCE_EXTRA_TABLES", []string{"contact_purchases"}),
		DefaultPageSize:           config.Int("PANEL_DEFAULT_PAGE_SIZE", 20),
		MaxPageSize:               config.Int("PANEL_MAX_PAGE_SIZE", 200),

		RateLimitEnabled:    config.Bool("API_RATE_LIMIT_ENABLED", true),
		RateLimitReadRPS:    config.Float("API_RATE_LIMIT_READ_RPS", 20),
		RateLimitReadBurst:  config.Int("API_RATE_LIMIT_READ_BURST", 40),
		RateLimitWriteRPS:   config.Float("API_RATE_LIMIT_WRITE_RPS", 5),
		RateLimitWriteBurst: config.Int("API_RATE_LIMIT_WRITE_BURST", 10),
		RateLimitTrustProxy: config.Bool("API_RATE_LIMIT_TRUST_PROXY", false),
	}
}
