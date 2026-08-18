package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/redis/go-redis/v9"
)

func main() {
	rebuildCache := flag.Bool("rebuild-cache", false, "rebuild disposable balance cache from PostgreSQL projections")
	runReconciliation := flag.Bool("run", false, "persist a projection-versus-latest-event reconciliation result")
	tenantID := flag.String("tenant-id", "", "tenant UUID required with --rebuild-cache")
	flag.Parse()
	if *tenantID == "" || (!*rebuildCache && !*runReconciliation) {
		slog.Error("usage: reconcile --run --tenant-id <uuid> [--rebuild-cache]")
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
	if *runReconciliation {
		repository, err := db.NewReconciliationRepository(database)
		if err != nil {
			slog.Error("reconciliation repository initialization failed", "error", err)
			os.Exit(1)
		}
		service, err := reconciliation.NewService(repository, nil)
		if err != nil {
			slog.Error("reconciliation service initialization failed", "error", err)
			os.Exit(1)
		}
		result, err := service.Run(ctx, *tenantID)
		if err != nil {
			slog.Error("reconciliation failed", "error", err)
			os.Exit(1)
		}
		slog.Info("reconciliation completed", "tenant_id", result.TenantID, "status", result.Status, "checked_accounts", result.CheckedAccountCount, "mismatch_count", result.MismatchCount, "run_id", result.ID)
		if result.Status == reconciliation.StatusMismatch {
			os.Exit(3)
		}
	}
	if !*rebuildCache {
		return
	}
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
