package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	if len(os.Args) == 2 && os.Args[1] == "--healthcheck" {
		if err := checkWorkerHealth(configuredWorkerHealthPath(), time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := runWorker(); err != nil {
		slog.Error("worker process failed", "error", err)
		os.Exit(1)
	}
}

// runWorker owns resources so initialization failures unwind their cleanup.
func runWorker() error {
	// A restarted process must prove its own polls before the probe can pass.
	healthPath := configuredWorkerHealthPath()
	_ = os.Remove(healthPath)
	defer func() { _ = os.Remove(healthPath) }()
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration invalid: %w", err)
	}
	telemetry, err := observability.NewTelemetry(context.Background(), observability.TelemetryConfig{Enabled: configuration.TelemetryEnabled, ServiceName: configuration.TelemetryServiceName, Endpoint: configuration.OTLPHTTPEndpoint, CAFile: configuration.OTLPCAFile})
	if err != nil {
		return fmt.Errorf("telemetry initialization failed: %w", err)
	}
	defer func() { _ = telemetry.Shutdown(context.Background()) }()
	if configuration.DatabaseURL == "" || configuration.RedisAddress == "" {
		return errors.New("outbox worker requires database and redis configuration")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	healthObserver := &workerHealthObserver{delegate: telemetry, path: healthPath, reports: make(map[string]observability.WorkerProgressReport)}
	progressMonitor, err := observability.NewWorkerProgressMonitor(healthObserver, 5*time.Second, nil)
	if err != nil {
		return fmt.Errorf("worker progress monitor initialization failed: %w", err)
	}
	go progressMonitor.Run(ctx)
	startupConfig := startup.Config{
		Timeout:        configuration.StartupTimeout,
		InitialBackoff: configuration.StartupInitialBackoff,
		MaxBackoff:     configuration.StartupMaxBackoff,
		OnRetry: func(event startup.Event) {
			slog.Warn("dependency not ready during bounded startup", "dependency", event.Dependency, "attempt", event.Attempt, "category", event.Category, "retry_in", event.Delay, "startup_time_remaining", event.Remaining, "error", event.Err)
		},
	}
	database, err := startup.Open(ctx, "postgresql", startupConfig, func(ctx context.Context) (*sql.DB, error) {
		return db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL, MaxOpenConns: configuration.DatabaseMaxOpenConns, ApplicationName: configuration.NodeIdentity("worker")})
	})
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
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
		return fmt.Errorf("redis initialization failed: %w", err)
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			slog.Warn("redis close failed", "error", closeErr)
		}
	}()
	runner, err := bootstrap.NewWorkerRunnerWithOptions(ctx, configuration, database, redisClient, telemetry, bootstrap.WorkerRunnerOptions{
		NodeRole: "worker", KeyCacheObserver: telemetry,
		ItemObserver: func(queue string) func(string) { return workerItemObserver(progressMonitor, queue) },
		OnStarted:    func(queue string) { markWorkerStarted(progressMonitor, queue) },
		OnCompleted: func(queue string, runErr error) {
			markWorkerCompleted(progressMonitor, queue, runErr)
			if runErr != nil && ctx.Err() == nil {
				slog.Error("worker queue iteration failed", "queue", queue, "error", runErr)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("worker processors initialization failed: %w", err)
	}
	poll := time.NewTicker(200 * time.Millisecond)
	defer poll.Stop()
	healthPoll := time.NewTicker(15 * time.Second)
	defer healthPoll.Stop()
	for {
		_, _ = runner.RunOnce(ctx)
		select {
		case <-ctx.Done():
			goto stopped
		case <-healthPoll.C:
			if healthErr := runner.ObserveHealth(ctx); healthErr != nil {
				slog.Warn("redis stream health observation failed", "error", healthErr)
			}
		case <-poll.C:
		}
	}
stopped:
	slog.Info("LedgerSync outbox worker stopped", "environment", configuration.Environment)
	return nil
}

func workerItemObserver(monitor *observability.WorkerProgressMonitor, queue string) func(string) {
	return func(itemID string) {
		if err := monitor.MarkItem(queue, itemID); err != nil {
			slog.Error("worker progress item update failed", "queue", queue, "error", err)
		}
	}
}

func markWorkerStarted(monitor *observability.WorkerProgressMonitor, queue string) {
	if err := monitor.MarkStarted(queue, ""); err != nil {
		slog.Error("worker progress start update failed", "queue", queue, "error", err)
	}
}

func markWorkerCompleted(monitor *observability.WorkerProgressMonitor, queue string, runErr error) {
	if err := monitor.MarkCompleted(queue, runErr != nil); err != nil {
		slog.Error("worker progress completion update failed", "queue", queue, "error", err)
	}
}
