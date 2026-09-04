package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/redisconn"
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
		options, optionsErr := redisconn.Options(configuration.RedisAddress)
		if optionsErr != nil {
			return nil, optionsErr
		}
		client := redis.NewClient(options)
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
	runner, err := bootstrap.NewWorkerRunner(ctx, configuration, database, redisClient, telemetry)
	if err != nil {
		slog.Error("worker initialization failed", "error", err)
		os.Exit(1)
	}

	poll := time.NewTicker(200 * time.Millisecond)
	defer poll.Stop()
	healthPoll := time.NewTicker(15 * time.Second)
	defer healthPoll.Stop()
	for {
		_, runErr := runner.RunOnce(ctx)
		if runErr != nil && ctx.Err() == nil {
			slog.Error("worker iteration failed", "error", runErr)
		}
		select {
		case <-ctx.Done():
			slog.Info("LedgerSync outbox worker stopped", "environment", configuration.Environment)
			return
		case <-healthPoll.C:
			if healthErr := runner.ObserveHealth(ctx); healthErr != nil {
				slog.Warn("redis stream health observation failed", "error", healthErr)
			}
		case <-poll.C:
		}
	}
}
