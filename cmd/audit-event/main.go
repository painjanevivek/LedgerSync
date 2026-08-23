// Command audit-event appends redacted operational evidence for privileged
// configuration, credential lifecycle, break-glass, and recovery actions.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

var allowedEvents = map[string]bool{
	"configuration.changed": true,
	"credential.rotated":    true,
	"recovery.drill":        true,
	"break_glass.used":      true,
}

func main() {
	tenant := flag.String("tenant-id", "", "tenant UUID")
	actor := flag.String("actor-subject-id", "", "OIDC or workload subject")
	eventType := flag.String("event-type", "", "approved operational event type")
	targetType := flag.String("target-type", "", "non-sensitive target category")
	targetID := flag.String("target-id", "", "non-sensitive target identifier")
	outcome := flag.String("outcome", "", "succeeded or failed")
	correlation := flag.String("correlation-id", "", "incident/change UUID")
	ticket := flag.String("ticket", "", "approved change or incident reference")
	flag.Parse()
	if !allowedEvents[*eventType] || (*outcome != "succeeded" && *outcome != "failed") || strings.TrimSpace(*tenant) == "" || strings.TrimSpace(*actor) == "" || strings.TrimSpace(*targetType) == "" || strings.TrimSpace(*correlation) == "" {
		fmt.Fprintln(os.Stderr, "valid tenant, actor, approved event type, target type, outcome, and correlation ID are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: os.Getenv("LEDGERSYNC_DATABASE_URL")})
	if err != nil {
		fail(err)
	}
	defer database.Close()
	repository, err := db.NewAuditRepository(database)
	if err != nil {
		fail(err)
	}
	err = repository.Record(ctx, db.AuditEvent{TenantID: *tenant, ActorSubjectID: *actor, EventType: *eventType, TargetType: *targetType, TargetID: *targetID, Outcome: *outcome, CorrelationID: *correlation, Metadata: map[string]string{"ticket": *ticket}, OccurredAt: time.Now().UTC()})
	if err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
