package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/softwarecase/event-driven-notification-service/internal/adapter/cache"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/event"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/handler"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/handler/middleware"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/queue"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/repository/postgres"
	"github.com/softwarecase/event-driven-notification-service/internal/config"
	"github.com/softwarecase/event-driven-notification-service/internal/service"
)

func main() {
	// Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Config
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
			logger.Warn("failed to init tracer, continuing without tracing", "error", err)
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

	if err := dbPool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to database")

	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() { _ = redisClient.Close() }()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to redis")

	// Adapters
	_ = cache.NewRedisCache(redisClient)
	notifRepo := postgres.NewNotificationRepo(dbPool)
	templateRepo := postgres.NewTemplateRepo(dbPool)
	publisher := queue.NewRedisPublisher(redisClient)
	consumer := queue.NewRedisConsumer(redisClient)
	hub := event.NewHub(logger, redisClient, true) // subscriber mode: receive Worker events via Redis

	// Start WebSocket hub + Redis subscriber for cross-process events
	go hub.Run()
	go hub.SubscribeRedis(ctx)

	// Services
	templateSvc := service.NewTemplateService(templateRepo, logger)
	notifSvc := service.NewNotificationService(notifRepo, templateSvc, publisher, hub, logger)

	// Handlers
	notifHandler := handler.NewNotificationHandler(notifSvc)
	templateHandler := handler.NewTemplateHandler(templateSvc)
	healthHandler := handler.NewHealthHandler(dbPool, redisClient)
	metricsHandler := handler.NewMetricsHandler(redisClient, consumer, nil)
	wsHandler := handler.NewWebSocketHandler(hub, logger)

	// Router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Tracing("notification-api"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Routes
	r.Get("/health", healthHandler.Check)
	r.Get("/metrics", metricsHandler.Get)
	r.Get("/ws/notifications", wsHandler.Handle)

	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/notifications", notifHandler.Routes())
		r.Mount("/templates", templateHandler.Routes())
	})

	// Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("api server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
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
			semconv.ServiceNameKey.String("notification-api"),
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
