package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func main() {
	action := flag.String("action", "inspect", "inspect, approve, or replay")
	tenant := flag.String("tenant-id", "", "tenant UUID")
	attempt := flag.String("attempt-id", "", "dead delivery attempt UUID")
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
	defer func() { _ = database.Close() }()
	repository, err := db.NewDeliveryReplayRepository(database, nil)
	if err != nil {
		fail(err)
	}
	switch *action {
	case "inspect":
		item, inspectErr := repository.Inspect(ctx, *tenant, *attempt)
		if inspectErr != nil {
			fail(inspectErr)
		}
		_ = json.NewEncoder(os.Stdout).Encode(item)
	case "approve":
		err = repository.Approve(ctx, recovery.DeliveryApproval{TenantID: *tenant, AttemptID: *attempt, ActorSubjectID: *actor, ReasonCode: *reason, CorrelationID: *correlation})
	case "replay":
		var newAttemptID string
		newAttemptID, err = repository.Replay(ctx, recovery.DeliveryReplay{TenantID: *tenant, AttemptID: *attempt, ActorSubjectID: *actor, CorrelationID: *correlation})
		if err == nil {
			fmt.Println(newAttemptID)
		}
	default:
		err = fmt.Errorf("unsupported action %q", *action)
	}
	if err != nil {
		telemetry.ObserveRecovery(ctx, "delivery", *action, err)
		fail(err)
	}
	telemetry.ObserveRecovery(ctx, "delivery", *action, nil)
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
