// Command smssender consumes the SMS queue (competing consumers: run as many
// replicas as needed, every batch is processed by exactly one of them) and
// delivers messages through the external provider.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"notifier/pkg/healthserver"
	"notifier/pkg/logger"
	"notifier/pkg/postgres"
	"notifier/pkg/rabbitmq"
	"notifier/pkg/ratelimit"
	"notifier/pkg/timezone"
	"notifier/pkg/tracing"
	"notifier/smssender/config"
	"notifier/smssender/consumer"
	"notifier/smssender/provider"
	"notifier/smssender/repository"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.ServiceName, cfg.LogLevel)
	if err := run(cfg, log); err != nil {
		log.Error("sender stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	if cfg.ProviderURL == "" {
		return errors.New("provider url is not configured (PROVIDER_URL)")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTELEndpoint, cfg.OTELEnabled)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	tz := timezone.NewHolder()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, postgres.Options{MaxConns: int32(cfg.Workers*2 + 4), ApplicationName: cfg.ServiceName, AfterConnect: tz.AfterConnect}, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := tz.Init(ctx, pool, log); err != nil {
		return err
	}
	go tz.Watch(ctx, pool, timezone.WatchInterval(), stop, log)

	mq, err := rabbitmq.NewClient(ctx, cfg.RabbitURL, log)
	if err != nil {
		return err
	}
	defer mq.Close()
	queue, err := rabbitmq.QueueForChannel(config.Channel)
	if err != nil {
		return err
	}

	handler := consumer.NewHandler(
		repository.NewDeliveryRepository(pool),
		repository.NewNotificationRepository(pool),
		repository.NewLogRepository(pool, cfg.ServiceName),
		provider.NewClient(cfg.ProviderURL, cfg.ProviderTimeout),
		ratelimit.New(pool, "channel:"+config.Channel, cfg.RateLimit),
		cfg, log,
	)

	go healthserver.Run(ctx, ":"+cfg.HealthPort, cfg.ServiceName, map[string]healthserver.Check{
		"postgres": pool.Ping,
		"rabbitmq": mq.Healthy,
		"timezone": tz.Check(pool),
	}, log)

	c := rabbitmq.NewConsumer(mq, rabbitmq.ConsumerConfig{
		Queue:    queue,
		Prefetch: cfg.Prefetch,
		Workers:  cfg.Workers,
	}, handler.Handle, log)

	log.Info("sender started", slog.String("queue", queue), slog.Int("rate_limit_per_second", cfg.RateLimit))
	if err := c.Run(ctx); err != nil {
		return err
	}
	if tz.RestartRequired() {
		return timezone.ErrRestartRequired
	}
	return nil
}
