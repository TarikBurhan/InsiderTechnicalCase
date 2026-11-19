// Command panel runs the Notification Management API: it validates incoming
// notification requests, stores them as NEW and exposes status, metrics,
// WebSocket updates and API docs.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notifier/panel/config"
	"notifier/panel/handlers"
	"notifier/panel/repository"
	"notifier/panel/server"
	"notifier/panel/service"
	"notifier/panel/websocket"
	"notifier/panel/worker"
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
		log.Error("panel stopped with error", slog.Any("error", err))
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

	// Every DB session and the process itself use the account time zone.
	tz := timezone.NewHolder()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL, postgres.Options{MaxConns: 20, ApplicationName: "panel", AfterConnect: tz.AfterConnect}, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	readerPool, err := postgres.NewPool(ctx, cfg.AudienceDatabaseURL, postgres.Options{MaxConns: 5, ApplicationName: "panel-audience", AfterConnect: tz.AfterConnect}, log)
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

	// Repositories
	notificationRepo := repository.NewNotificationRepository(pool)
	deliveryRepo := repository.NewDeliveryRepository(pool)
	audienceRepo := repository.NewAudienceRepository(pool, readerPool)
	contentRepo := repository.NewContentRepository(pool)
	logRepo := repository.NewLogRepository(pool, cfg.ServiceName)
	metricsRepo := repository.NewMetricsRepository(pool)
	settingRepo := repository.NewSettingRepository(pool)
	calcRepo := repository.NewAudienceCalculationRepository(pool)

	// Services
	audienceSvc := service.NewAudienceService(audienceRepo, service.AudienceConfig{
		ValidationTimeout:   cfg.AudienceValidationTimeout,
		ExtraTables:         cfg.AudienceExtraTables,
		MaxDirectRecipients: cfg.MaxDirectRecipients,
	})
	contentSvc := service.NewContentService(contentRepo, audienceSvc)

	// Background work: audiences that could not be counted within the API
	// timeout are finished here. The watchdog marks the Panel as stuck when
	// the loop stops ticking or a calculation makes no progress.
	watchdog := healthserver.NewWatchdog(cfg.AudienceBackgroundTimeout + 5*time.Minute)
	calculator := worker.NewAudienceCalculator(calcRepo, audienceSvc, logRepo, worker.AudienceCalculatorConfig{
		Workers:      cfg.AudienceCalcWorkers,
		PollInterval: cfg.AudienceCalcPollInterval,
		Lease:        cfg.AudienceCalcLease,
		Timeout:      cfg.AudienceBackgroundTimeout,
	}, watchdog, log)
	go calculator.Run(ctx)

	notificationSvc := service.NewNotificationService(notificationRepo, deliveryRepo, logRepo, audienceSvc, contentSvc,
		calculator, service.NotificationConfig{MaxBatchSize: cfg.MaxBatchSize}, log)
	metricsSvc := service.NewMetricsService(metricsRepo, mq)
	settingSvc := service.NewSettingService(settingRepo, tz, log)

	// Real-time status updates (PostgreSQL LISTEN/NOTIFY -> WebSocket)
	hub := websocket.NewHub()
	go websocket.NewListener(pool, hub, log).Run(ctx)

	limiter := server.NewRateLimiter(server.RateLimitConfig{
		Enabled:    cfg.RateLimitEnabled,
		ReadRPS:    cfg.RateLimitReadRPS,
		ReadBurst:  cfg.RateLimitReadBurst,
		WriteRPS:   cfg.RateLimitWriteRPS,
		WriteBurst: cfg.RateLimitWriteBurst,
		TrustProxy: cfg.RateLimitTrustProxy,
	})
	go limiter.RunJanitor(ctx)

	paging := handlers.Paging{Default: cfg.DefaultPageSize, Max: cfg.MaxPageSize}
	router := server.NewRouter(server.Handlers{
		Notifications: handlers.NewNotificationHandler(notificationSvc, paging, log),
		Audiences:     handlers.NewAudienceHandler(audienceSvc, paging, log),
		Contents:      handlers.NewContentHandler(contentSvc, paging, log),
		Metrics:       handlers.NewMetricsHandler(metricsSvc, hub, log),
		WebSocket:     handlers.NewWebSocketHandler(hub, log),
		Settings:      handlers.NewSettingHandler(settingSvc, log),
		RateLimiter:   limiter,
		ReadyChecks: map[string]healthserver.Check{
			"postgres": pool.Ping,
			"rabbitmq": mq.Healthy,
			"timezone": tz.Check(pool),
		},
		LiveChecks: map[string]healthserver.Check{"watchdog": watchdog.Check},
	}, log)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("panel listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down panel")
	case err := <-errCh:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if tz.RestartRequired() {
		return timezone.ErrRestartRequired
	}
	return nil
}
