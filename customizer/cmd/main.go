// Command customizer consumes batches whose content contains {{variables}},
// renders the content per recipient from master_contact and forwards the
// batch to the channel sender queue.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"notifier/customizer/config"
	"notifier/customizer/consumer"
	"notifier/customizer/repository"
	"notifier/pkg/healthserver"
	"notifier/pkg/logger"
	"notifier/pkg/postgres"
	"notifier/pkg/rabbitmq"
	"notifier/pkg/timezone"
	"notifier/pkg/tracing"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.ServiceName, cfg.LogLevel)
	if err := run(cfg, log); err != nil {
		log.Error("customizer stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTELEndpoint, cfg.OTELEnabled)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	tz := timezone.NewHolder()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, postgres.Options{MaxConns: int32(cfg.Workers + 4), ApplicationName: "customizer", AfterConnect: tz.AfterConnect}, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	readerPool, err := postgres.NewPool(ctx, cfg.AudienceDatabaseURL, postgres.Options{MaxConns: int32(cfg.Workers + 2), ApplicationName: "customizer-contacts", AfterConnect: tz.AfterConnect}, log)
	if err != nil {
		return err
	}
	defer readerPool.Close()
	if err := tz.Init(ctx, pool, log, readerPool); err != nil {
		return err
	}
	go tz.Watch(ctx, pool, timezone.WatchInterval(), stop, log)

	mq, err := rabbitmq.NewClient(ctx, cfg.RabbitURL, log)
	if err != nil {
		return err
	}
	defer mq.Close()
	if err := rabbitmq.SetupTopology(mq); err != nil {
		return err
	}
	pub := rabbitmq.NewPublisher(mq)
	defer pub.Close()

	handler := consumer.NewHandler(
		repository.NewContactRepository(readerPool),
		repository.NewDeliveryRepository(pool),
		repository.NewLogRepository(pool, cfg.ServiceName),
		pub, log,
	)

	go healthserver.Run(ctx, ":"+cfg.HealthPort, cfg.ServiceName, map[string]healthserver.Check{
		"postgres": pool.Ping,
		"rabbitmq": mq.Healthy,
		"timezone": tz.Check(pool),
	}, log)

	c := rabbitmq.NewConsumer(mq, rabbitmq.ConsumerConfig{
		Queue:          rabbitmq.QueueCustomizer,
		Prefetch:       cfg.Prefetch,
		Workers:        cfg.Workers,
		HandlerTimeout: cfg.HandlerTimeout,
	}, handler.Handle, log)

	log.Info("customizer started")
	if err := c.Run(ctx); err != nil {
		return err
	}
	if tz.RestartRequired() {
		return timezone.ErrRestartRequired
	}
	return nil
}
