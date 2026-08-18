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
	"github.com/redis/go-redis/v9"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
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
	store, err := db.NewOutboxRepository(database, nil)
	if err != nil {
		slog.Error("outbox repository initialization failed", "error", err)
		os.Exit(1)
	}
	streams, err := events.NewRedisStreams(redisClient, "")
	if err != nil {
		slog.Error("redis streams initialization failed", "error", err)
		os.Exit(1)
	}
	hostname, _ := os.Hostname()
	worker, err := outbox.NewWorker(store, streams, nil, nil, outbox.Config{WorkerID: fmt.Sprintf("%s-%d", hostname, os.Getpid())})
	if err != nil {
		slog.Error("outbox worker initialization failed", "error", err)
		os.Exit(1)
	}
	balanceCache, err := cacheplatform.NewBalanceCache(redisClient, "", 5*time.Minute)
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
	for {
		if _, err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("outbox publish iteration failed", "error", err)
		}
		if _, err := projector.RunOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("balance projection iteration failed", "error", err)
		}
		select {
		case <-ctx.Done():
			goto stopped
		case <-poll.C:
		}
	}
stopped:
	slog.Info("LedgerSync outbox worker stopped", "environment", configuration.Environment)
}
