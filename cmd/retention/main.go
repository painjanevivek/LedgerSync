package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/retention"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func main() {
	tenant := flag.String("tenant-id", "", "tenant UUID")
	correlation := flag.String("correlation-id", "", "approved change UUID")
	apply := flag.Bool("apply", false, "apply one bounded batch; default is dry-run")
	batch := flag.Int("batch-size", 500, "maximum rows per eligible class")
	outboxDays := flag.Int("published-outbox-days", 30, "retain published outbox rows for at least this many days")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	telemetry, err := observability.NewTelemetry(ctx, observability.TelemetryConfig{Enabled: os.Getenv("LEDGERSYNC_TELEMETRY_ENABLED") == "true", ServiceName: os.Getenv("LEDGERSYNC_TELEMETRY_SERVICE_NAME"), Endpoint: os.Getenv("LEDGERSYNC_OTLP_HTTP_ENDPOINT")})
	if err != nil {
		fail(err)
	}
	defer func() { _ = telemetry.Shutdown(context.Background()) }()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: os.Getenv("LEDGERSYNC_DATABASE_URL")})
	if err != nil {
		fail(err)
	}
	defer database.Close()
	repository, err := db.NewRetentionRepository(database, nil)
	if err != nil {
		fail(err)
	}
	result, err := repository.Run(ctx, retention.Policy{TenantID: *tenant, BatchSize: *batch, PublishedOutboxAfter: time.Duration(*outboxDays) * 24 * time.Hour, RateWindowAfter: 2 * time.Hour}, *apply, *correlation)
	if err != nil {
		fail(err)
	}
	telemetry.ObserveRetention(ctx, result.Mode, result.PublishedOutbox+result.ExpiredRates)
	_ = json.NewEncoder(os.Stdout).Encode(result)
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
