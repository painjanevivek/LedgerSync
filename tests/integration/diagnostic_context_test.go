package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestDiagnosticOriginSurvivesScopedCommitAndWorkerRetry(t *testing.T) {
	service, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	provider := sdktrace.NewTracerProvider()
	ctx, span := provider.Tracer("diagnostic-test").Start(context.Background(), "fixture")
	origin := observability.TraceParent(ctx)
	if _, err := service.Submit(ctx, transferCommand(t, "diagnostic-origin-request-001", "1.00")); err != nil {
		t.Fatal(err)
	}
	span.End()
	var count int
	if err := admin.QueryRow(`SELECT count(*) FROM outbox_events WHERE substring(diagnostic_traceparent from 4 for 32)=$1`, origin[3:35]).Scan(&count); err != nil || count != 3 {
		t.Fatalf("persisted origins=%d error=%v", count, err)
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	store, _ := db.NewOutboxRepository(worker.db, nil)
	clock, _ := db.NewLogicalClockRepository(worker.db, "ledgersync/worker/r06")
	events, err := store.Claim(context.Background(), "diagnostic-worker", 10, time.Minute)
	if err != nil || len(events) != 3 {
		t.Fatalf("claim: %d %v", len(events), err)
	}
	for _, event := range events {
		first, err := clock.PreparePublication(context.Background(), "diagnostic-worker", event)
		if err != nil {
			t.Fatal(err)
		}
		second, err := clock.PreparePublication(context.Background(), "diagnostic-worker", event)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(first.DiagnosticTraceParent, origin[3:35]) || first.DiagnosticTraceParent != second.DiagnosticTraceParent || first.ID != second.ID {
			t.Fatal("retry changed immutable origin or event identity")
		}
	}
	if _, err := admin.Exec(`UPDATE outbox_events SET diagnostic_traceparent=NULL WHERE id=$1`, events[0].ID); err == nil {
		t.Fatal("origin evidence was mutable")
	}
	// A connection reused without trace context must not inherit this origin.
	if _, err := service.Submit(context.Background(), transferCommand(t, "diagnostic-no-context-002", "1.00")); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE diagnostic_traceparent IS NULL`); got != 3 {
		t.Fatalf("scope leaked: null origins=%d", got)
	}
}
