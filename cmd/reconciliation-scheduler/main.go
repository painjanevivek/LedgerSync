package main

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/scheduler"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("scheduler stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	if os.Getenv("LEDGERSYNC_SCHEDULER_ENABLED") != "true" {
		slog.Info("scheduled reconciliation disabled")
		return nil
	}
	tenants := strings.Split(os.Getenv("LEDGERSYNC_SCHEDULER_TENANTS"), ",")
	if len(tenants) == 0 || len(tenants) > 10 {
		return errors.New("one to ten scheduler tenants are required")
	}
	for i, tenant := range tenants {
		canonical, err := identifier.Parse(context.Background(), identifier.KindTenant, strings.TrimSpace(tenant))
		if err != nil {
			return errors.New("invalid scheduler tenant")
		}
		tenants[i] = canonical.String()
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dsn, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid scheduler database configuration")
	}
	query := dsn.Query()
	query.Set("options", "-c search_path=ledgersync_scheduler,public")
	dsn.RawQuery = query.Encode()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, startupCancel := context.WithTimeout(ctx, 90*time.Second)
	defer startupCancel()
	database, err := db.OpenPool(startup, db.PoolConfig{DriverName: "pgx", DSN: dsn.String(), MaxOpenConns: 4, ApplicationName: cfg.NodeIdentity("scheduler")})
	if err != nil {
		return errors.New("scheduler database unavailable")
	}
	defer database.Close()
	repository, _ := db.NewSchedulerRepository(database)
	reconciliationRepository, _ := db.NewReconciliationRepository(database)
	runner, _ := reconciliation.NewService(reconciliationRepository, nil)
	leaseDuration := 15 * time.Second
	if raw := os.Getenv("LEDGERSYNC_SCHEDULER_LEASE"); raw != "" {
		leaseDuration, err = time.ParseDuration(raw)
		if err != nil {
			return errors.New("invalid scheduler lease")
		}
	}
	interval := 5 * time.Minute
	if raw := os.Getenv("LEDGERSYNC_SCHEDULER_INTERVAL"); raw != "" {
		interval, err = time.ParseDuration(raw)
		if err != nil {
			return errors.New("invalid scheduler interval")
		}
	}
	service, err := scheduler.New(repository, runner, cfg.NodeIdentity("scheduler"), leaseDuration, interval)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		for _, tenant := range tenants {
			cycleCtx, cycleCancel := context.WithTimeout(ctx, 30*time.Second)
			err := service.Tick(cycleCtx, tenant)
			cycleCancel()
			if err != nil {
				slog.Warn("scheduler cycle unavailable")
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
