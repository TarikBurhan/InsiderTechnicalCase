// Package config loads the Customizer service configuration.
package config

import (
	"time"

	"notifier/pkg/config"
	"notifier/pkg/postgres"
	"notifier/pkg/rabbitmq"
)

// Config holds every Customizer setting.
type Config struct {
	ServiceName string
	LogLevel    string
	HealthPort  string

	DatabaseURL         string
	AudienceDatabaseURL string
	RabbitURL           string

	OTELEnabled  bool
	OTELEndpoint string

	Workers        int
	Prefetch       int
	HandlerTimeout time.Duration
}

// Load reads configuration from environment variables.
func Load() Config {
	host := config.String("POSTGRES_HOST", "localhost")
	port := config.String("POSTGRES_PORT", "5432")
	db := config.String("POSTGRES_DB", "notifications")
	ssl := config.String("POSTGRES_SSLMODE", "disable")

	return Config{
		ServiceName: "customizer",
		LogLevel:    config.String("LOG_LEVEL", "info"),
		HealthPort:  config.String("CUSTOMIZER_HEALTH_PORT", "8082"),

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

		Workers:        config.Int("CUSTOMIZER_WORKERS", 4),
		Prefetch:       config.Int("CUSTOMIZER_PREFETCH", 8),
		HandlerTimeout: config.Duration("CUSTOMIZER_HANDLER_TIMEOUT", 2*time.Minute),
	}
}
