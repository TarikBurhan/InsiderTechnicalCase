// Command manager is the processing engine: it polls NEW (and due scheduled)
// notifications, resolves audiences, writes per-channel delivery rows and
// produces batches of <= 100 recipients to RabbitMQ. It also recovers
// crashed work, schedules retries and completes finished notifications.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"notifier/manager/config"
	"notifier/manager/publisher"
	"notifier/manager/repository"
	"notifier/manager/worker"
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
		log.Error("manager stopped with error", slog.Any("error", err))
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
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, postgres.Options{MaxConns: int32(cfg.Workers*2 + 10), ApplicationName: "manager", AfterConnect: tz.AfterConnect}, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	readerPool, err := postgres.NewPool(ctx, cfg.AudienceDatabaseURL, postgres.Options{MaxConns: int32(cfg.Workers + 2), ApplicationName: "manager-audience", AfterConnect: tz.AfterConnect}, log)
	if err != nil {
		return err
	}
	defer readerPool.Close()
	if err := tz.Init(ctx, pool, log, readerPool); err != nil {
		return err
	}

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
	batchPublisher := publisher.New(pub)

	notifications := repository.NewNotificationRepository(pool)
	deliveries := repository.NewDeliveryRepository(pool)
	audience := repository.NewAudienceRepository(readerPool, cfg.AudienceQueryTimeout)
	logs := repository.NewLogRepository(pool, cfg.ServiceName)

	processor := worker.NewProcessor(notifications, deliveries, audience, logs, batchPublisher, worker.ProcessorConfig{
		BatchSize:             cfg.BatchSize,
		HeartbeatInterval:     cfg.HeartbeatInterval,
		MaxProcessingAttempts: cfg.MaxProcessingAttempts,
	}, log)
	dispatcher := worker.NewDispatcher(notifications, processor, worker.DispatcherConfig{
		PollInterval:         cfg.PollInterval,
		Workers:              cfg.Workers,
		StaleProcessingAfter: cfg.StaleProcessingAfter,
		RecoveryInterval:     cfg.RecoveryInterval,
	}, log)
	retries := worker.NewRetryScheduler(notifications, deliveries, logs, batchPublisher,
		cfg.RetryPollInterval, cfg.RetryClaimLimit, cfg.BatchSize, log)
	finalizer := worker.NewFinalizer(notifications, logs, cfg.FinalizerInterval, log)
	sweeper := worker.NewSweeper(deliveries, logs, cfg.SweeperInterval, cfg.StaleSendingAfter, cfg.SweeperClaimLimit, log)

	// Every loop runs in its own goroutine so none of them blocks another.
	var wg sync.WaitGroup
	start := func(fn func(context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(ctx)
		}()
	}
	start(func(ctx context.Context) { tz.Watch(ctx, pool, timezone.WatchInterval(), stop, log) })
	start(dispatcher.Run)
	start(retries.Run)
	start(finalizer.Run)
	start(sweeper.Run)
	start(func(ctx context.Context) {
		healthserver.Run(ctx, ":"+cfg.HealthPort, cfg.ServiceName, map[string]healthserver.Check{
			"postgres": pool.Ping,
			"rabbitmq": mq.Healthy,
			"timezone": tz.Check(pool),
		}, log)
	})

	log.Info("manager started")
	<-ctx.Done()
	log.Info("shutting down manager")
	wg.Wait()
	if tz.RestartRequired() {
		return timezone.ErrRestartRequired
	}
	return nil
}
