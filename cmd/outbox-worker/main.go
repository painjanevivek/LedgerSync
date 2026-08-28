package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/startup"
	"github.com/redis/go-redis/v9"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	telemetry, err := observability.NewTelemetry(context.Background(), observability.TelemetryConfig{Enabled: configuration.TelemetryEnabled, ServiceName: configuration.TelemetryServiceName, Endpoint: configuration.OTLPHTTPEndpoint})
	if err != nil {
		slog.Error("telemetry initialization failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = telemetry.Shutdown(context.Background()) }()
	if configuration.DatabaseURL == "" || configuration.RedisAddress == "" {
		slog.Error("outbox worker requires database and redis configuration")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	startupConfig := startup.Config{
		Timeout:        configuration.StartupTimeout,
		InitialBackoff: configuration.StartupInitialBackoff,
		MaxBackoff:     configuration.StartupMaxBackoff,
		OnRetry: func(event startup.Event) {
			slog.Warn("dependency not ready during bounded startup", "dependency", event.Dependency, "attempt", event.Attempt, "category", event.Category, "retry_in", event.Delay, "startup_time_remaining", event.Remaining, "error", event.Err)
		},
	}
	database, err := startup.Open(ctx, "postgresql", startupConfig, func(ctx context.Context) (*sql.DB, error) {
		return db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL})
	})
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			slog.Warn("database close failed", "error", closeErr)
		}
	}()
	redisClient, err := startup.Open(ctx, "redis", startupConfig, func(ctx context.Context) (*redis.Client, error) {
		client := redis.NewClient(&redis.Options{Addr: configuration.RedisAddress})
		if pingErr := client.Ping(ctx).Err(); pingErr != nil {
			_ = client.Close()
			return nil, pingErr
		}
		return client, nil
	})
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			slog.Warn("redis close failed", "error", closeErr)
		}
	}()
	store, err := db.NewOutboxRepository(database, nil, telemetry)
	if err != nil {
		slog.Error("outbox repository initialization failed", "error", err)
		os.Exit(1)
	}
	streams, err := events.NewRedisStreams(redisClient, "", telemetry)
	if err != nil {
		slog.Error("redis streams initialization failed", "error", err)
		os.Exit(1)
	}
	streams.WithMaxLength(configuration.RedisStreamMaxLength)
	hostname, _ := os.Hostname()
	ryewMetrics := observability.NewRYEWMetrics(telemetry)
	worker, err := outbox.NewWorker(store, streams, ryewMetrics, nil, outbox.Config{WorkerID: fmt.Sprintf("%s-%d", hostname, os.Getpid())})
	if err != nil {
		slog.Error("outbox worker initialization failed", "error", err)
		os.Exit(1)
	}
	webhookStore, err := db.NewWebhookDeliveryJobRepository(database, nil)
	if err != nil {
		slog.Error("webhook delivery store initialization failed", "error", err)
		os.Exit(1)
	}
	webhookKeys, err := webhookdelivery.NewStaticKeyResolver(configuration.WebhookSigningKeys)
	if err != nil {
		slog.Error("webhook signing key configuration invalid", "error", err)
		os.Exit(1)
	}
	webhookDispatcher, err := webhookdelivery.NewDispatcher(webhookdelivery.NewSecureHTTPClient(), webhookKeys, nil)
	if err != nil {
		slog.Error("webhook dispatcher initialization failed", "error", err)
		os.Exit(1)
	}
	webhookWorker, err := webhookdelivery.NewWorker(webhookStore, webhookDispatcher, nil, webhookdelivery.Config{WorkerID: fmt.Sprintf("%s-%d-webhooks", hostname, os.Getpid())})
	if err != nil {
		slog.Error("webhook worker initialization failed", "error", err)
		os.Exit(1)
	}
	balanceCache, err := cacheplatform.NewBalanceCache(redisClient, "", 5*time.Minute, telemetry)
	if err != nil {
		slog.Error("balance cache initialization failed", "error", err)
		os.Exit(1)
	}
	cacheAdapter, err := cacheplatform.NewAccountAdapter(balanceCache)
	if err != nil {
		slog.Error("balance cache adapter initialization failed", "error", err)
		os.Exit(1)
	}
	projector, err := projection.NewBalanceProjector(streams, cacheAdapter, projection.Config{Group: "balance-cache-v1", Consumer: fmt.Sprintf("%s-%d", hostname, os.Getpid())})
	if err != nil {
		slog.Error("balance projector initialization failed", "error", err)
		os.Exit(1)
	}
	poll := time.NewTicker(200 * time.Millisecond)
	defer poll.Stop()
	healthPoll := time.NewTicker(15 * time.Second)
	defer healthPoll.Stop()
	for {
		iterationStarted := time.Now()
		iterationCtx, span := telemetry.Start(ctx, "outbox.worker.publish")
		_, publishErr := worker.RunOnce(iterationCtx)
		span.End()
		telemetry.ObserveBoundary(iterationCtx, "worker", "publish", iterationStarted, publishErr)
		if publishErr != nil && ctx.Err() == nil {
			slog.Error("outbox publish iteration failed", "error", publishErr)
		}
		iterationStarted = time.Now()
		iterationCtx, span = telemetry.Start(ctx, "outbox.worker.webhook_dispatch")
		_, webhookErr := webhookWorker.RunOnce(iterationCtx)
		span.End()
		telemetry.ObserveBoundary(iterationCtx, "worker", "webhook_dispatch", iterationStarted, webhookErr)
		if webhookErr != nil && ctx.Err() == nil {
			slog.Error("webhook delivery iteration failed", "error", webhookErr)
		}
		iterationStarted = time.Now()
		iterationCtx, span = telemetry.Start(ctx, "outbox.worker.project")
		_, projectErr := projector.RunOnce(iterationCtx)
		span.End()
		telemetry.ObserveBoundary(iterationCtx, "worker", "project", iterationStarted, projectErr)
		if projectErr != nil && ctx.Err() == nil {
			slog.Error("balance projection iteration failed", "error", projectErr)
		}
		select {
		case <-ctx.Done():
			goto stopped
		case <-healthPoll.C:
			if healthErr := streams.ObserveHealth(ctx, "balance-cache-v1"); healthErr != nil {
				slog.Warn("redis stream health observation failed", "error", healthErr)
			}
		case <-poll.C:
		}
	}
stopped:
	slog.Info("LedgerSync outbox worker stopped", "environment", configuration.Environment)
}
