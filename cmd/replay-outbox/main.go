package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func main() {
	action := flag.String("action", "inspect", "inspect, approve, or replay")
	tenant := flag.String("tenant-id", "", "tenant UUID")
	event := flag.String("event-id", "", "dead outbox event UUID")
	actor := flag.String("actor-subject-id", "", "trusted operator subject")
	reason := flag.String("reason-code", "", "approved reason code")
	correlation := flag.String("correlation-id", "", "approval UUID")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			slog.Warn("database close failed", "error", closeErr)
		}
	}()
	repository, err := db.NewOutboxReplayRepository(database, nil)
	if err != nil {
		fail(err)
	}
	switch *action {
	case "inspect":
		item, err := repository.Inspect(ctx, *tenant, *event)
		if err != nil {
			fail(err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(item)
	case "approve":
		err = repository.Approve(ctx, recovery.Approval{TenantID: *tenant, EventID: *event, ActorSubjectID: *actor, ReasonCode: *reason, CorrelationID: *correlation})
	case "replay":
		err = repository.Replay(ctx, recovery.Replay{TenantID: *tenant, EventID: *event, ActorSubjectID: *actor, CorrelationID: *correlation})
	default:
		err = fmt.Errorf("unsupported action %q", *action)
	}
	if err != nil {
		telemetry.ObserveRecovery(ctx, "outbox", *action, err)
		fail(err)
	}
	telemetry.ObserveRecovery(ctx, "outbox", *action, nil)
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
