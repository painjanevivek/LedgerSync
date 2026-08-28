package operations

import (
	"context"
	"errors"
	"testing"
	"time"
)

type diagnosticRepositoryStub struct {
	facts DatabaseFacts
	err   error
	block bool
}

func (r diagnosticRepositoryStub) Facts(context.Context, string) (DatabaseFacts, error) {
	if r.block {
		select {}
	}
	return r.facts, r.err
}

type probeStub struct {
	err   error
	block bool
}

func (p probeStub) Ping(context.Context) error {
	if p.block {
		select {}
	}
	return p.err
}

func TestDiagnosticsPreservesFinancialAndDisposableDependencyDistinction(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	service, err := NewDiagnosticService(diagnosticRepositoryStub{facts: DatabaseFacts{SchemaVersion: "15", PendingOutboxCount: 2, DeadOutboxCount: 1, OldestPendingAt: now.Add(-3 * time.Minute), ReconciliationID: "run", ReconciliationStatus: "matched", ReconciledAt: now.Add(-time.Minute)}}, probeStub{err: errors.New("redis unavailable")}, BuildFacts{Version: "v1", Commit: "abc", Environment: "development"}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	result := service.Snapshot(context.Background(), "tenant")
	if result.OverallState != "degraded" || result.FinancialAuthority.PostgreSQL.State != "reachable" || result.DeliveryCache.Redis.State != "unavailable" || result.DeliveryCache.Redis.Label != "disposable_cache" || result.DeliveryCache.Outbox.WorkerProgress != "stalled" || result.Application.Environment != "local_demo" {
		t.Fatalf("unexpected partial diagnostics: %#v", result)
	}
}

func TestDiagnosticsReturnsWhenProbeIgnoresCancellation(t *testing.T) {
	service, err := NewDiagnosticService(diagnosticRepositoryStub{block: true}, probeStub{}, BuildFacts{Environment: "development"}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	service.timeout = 10 * time.Millisecond
	started := time.Now()
	result := service.Snapshot(context.Background(), "tenant")
	if time.Since(started) > 250*time.Millisecond || result.OverallState != "unavailable" || result.DeliveryCache.Redis.State != "reachable" {
		t.Fatalf("diagnostics did not return a bounded partial result: elapsed=%s result=%#v", time.Since(started), result)
	}
}

func TestBuildFactsRedactUnboundedOrControlValues(t *testing.T) {
	service, err := NewDiagnosticService(diagnosticRepositoryStub{}, probeStub{}, BuildFacts{Version: "postgres://user:secret@host/db", Commit: "abc\nsecret", Environment: "production"}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if service.build.Version != "development" || service.build.Commit != "unknown" || service.build.Environment != "configured" {
		t.Fatalf("unsafe build facts retained: %#v", service.build)
	}
}
