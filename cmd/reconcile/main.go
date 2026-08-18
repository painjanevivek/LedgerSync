package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/redis/go-redis/v9"
)

func main() {
	rebuildCache := flag.Bool("rebuild-cache", false, "rebuild disposable balance cache from PostgreSQL projections")
	tenantID := flag.String("tenant-id", "", "tenant UUID required with --rebuild-cache")
	flag.Parse()
	if !*rebuildCache || *tenantID == "" {
		slog.Error("usage: reconcile --rebuild-cache --tenant-id <uuid>")
		os.Exit(2)
	}
	configuration, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	if configuration.DatabaseURL == "" || configuration.RedisAddress == "" {
		slog.Error("cache rebuild requires database and redis configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL})
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	repository, err := db.NewBalanceRepository(database)
	if err != nil {
		slog.Error("balance repository initialization failed", "error", err)
		os.Exit(1)
	}
	balances, err := repository.ListCurrentForTenant(ctx, *tenantID)
	if err != nil {
		slog.Error("read authoritative projections failed", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: configuration.RedisAddress})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}
	cache, err := cacheplatform.NewBalanceCache(redisClient, "", 5*time.Minute)
	if err != nil {
		slog.Error("balance cache initialization failed", "error", err)
		os.Exit(1)
	}
	adapter, err := cacheplatform.NewAccountAdapter(cache)
	if err != nil {
		slog.Error("balance cache adapter initialization failed", "error", err)
		os.Exit(1)
	}
	for _, balance := range balances {
		if _, err := adapter.Put(ctx, balance); err != nil {
			slog.Error("cache rebuild failed", "account_id", balance.AccountID, "error", err)
			os.Exit(1)
		}
	}
	slog.Info("balance cache rebuilt from PostgreSQL projections", "tenant_id", *tenantID, "account_count", len(balances))
}
