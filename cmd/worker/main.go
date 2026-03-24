package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/softwarecase/event-driven-notification-service/internal/adapter/event"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/handler"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/provider"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/queue"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/repository/postgres"
	"github.com/softwarecase/event-driven-notification-service/internal/config"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/service"
	"github.com/softwarecase/event-driven-notification-service/pkg/ratelimit"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Tracing
	if cfg.Tracing.Enabled {
		tp, err := initTracer(ctx, cfg.Tracing.Endpoint)
		if err != nil {
			logger.Warn("failed to init tracer", "error", err)
		} else {
			defer func() { _ = tp.Shutdown(ctx) }()
		}
	}

	// Database
	dbPool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()
	logger.Info("connected to database")

	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() { _ = redisClient.Close() }()
	logger.Info("connected to redis")

	// Adapters
	notifRepo := postgres.NewNotificationRepo(dbPool)
	attemptRepo := postgres.NewDeliveryAttemptRepo(dbPool)
	dlqRepo := postgres.NewDeadLetterRepo(dbPool)
	publisher := queue.NewRedisPublisher(redisClient)
	consumer := queue.NewRedisConsumer(redisClient)
	webhookProvider := provider.NewWebhookProvider(cfg.Provider.WebhookURL, cfg.Provider.Timeout)
	limiter := ratelimit.NewLimiter(redisClient, cfg.Worker.RateLimitPerSec)
	hub := event.NewHub(logger, redisClient, false) // publisher mode: send events to Redis for API

	go hub.Run()

	// Metrics
	metricsCollector := handler.NewMetricsCollector(redisClient)

	// Services
	deliverySvc := service.NewDeliveryService(
		notifRepo, attemptRepo, dlqRepo,
		webhookProvider, publisher, hub, limiter, metricsCollector, logger,
		service.DeliveryConfig{
			MaxRetries: cfg.Worker.MaxRetries,
			BaseDelay:  cfg.Worker.RetryBaseDelay,
			MaxDelay:   cfg.Worker.RetryMaxDelay,
		},
	)

	scheduler := service.NewSchedulerService(notifRepo, publisher, logger, cfg.Worker.SchedulerInterval)
	retryPoller := service.NewRetryPoller(notifRepo, publisher, logger, 5*time.Second)

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	// Start scheduler and retry poller
	go scheduler.Run(ctx)
	go retryPoller.Run(ctx)

	// Start workers
	channels := []domain.Channel{domain.ChannelSMS, domain.ChannelEmail, domain.ChannelPush}
	var wg sync.WaitGroup

	logger.Info("starting workers",
		"concurrency", cfg.Worker.Concurrency,
		"channels", len(channels),
	)

	for _, ch := range channels {
		for i := 0; i < cfg.Worker.Concurrency; i++ {
			wg.Add(1)
			go func(channel domain.Channel, workerID int) {
				defer wg.Done()
				runWorker(ctx, channel, workerID, consumer, deliverySvc, logger, cfg.Worker.PollInterval)
			}(ch, i)
		}
	}

	<-done
	logger.Info("shutting down worker...")
	cancel()
	wg.Wait()
	logger.Info("worker stopped")
}

func runWorker(
	ctx context.Context,
	channel domain.Channel,
	workerID int,
	consumer *queue.RedisConsumer,
	delivery *service.DeliveryService,
	logger *slog.Logger,
	pollInterval time.Duration,
) {
	logger.Info("worker started", "channel", channel, "worker_id", workerID)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg, err := consumer.Dequeue(ctx, channel)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("dequeue error", "channel", channel, "worker_id", workerID, "error", err)
				continue
			}
			if msg == nil {
				continue // empty queue
			}

			if err := delivery.Process(ctx, msg.NotificationID); err != nil {
				logger.Error("delivery failed",
					"notification_id", msg.NotificationID,
					"channel", channel,
					"worker_id", workerID,
					"error", err,
				)
			}

			// Acknowledge
			_ = consumer.Acknowledge(ctx, channel, msg.NotificationID)
		}
	}
}

func initTracer(ctx context.Context, endpoint string) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceNameKey.String("notification-worker"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}
