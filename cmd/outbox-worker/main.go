package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
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
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL})
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: configuration.RedisAddress})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}
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
