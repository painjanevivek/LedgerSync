package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	appexports "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/exports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantEvidenceReadsAndExportsUseScopedConnections(t *testing.T) {
	service, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transfer, err := service.Submit(ctx, transferCommand(t, "tenant-evidence-context-001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	reconciler, _ := db.NewReconciliationRepository(admin)
	run, err := reconciler.Reconcile(ctx, testTenantID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"transfers", "journal_transactions", "ledger_postings", "reconciliation_runs", "reconciliation_mismatches", "outbox_events"} {
		if _, err := admin.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ENABLE ROW LEVEL SECURITY; CREATE POLICY portfolio_evidence_scope ON %s AS RESTRICTIVE USING (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid AND current_setting('app.actor_id',true)='%s')`, table, table, testActorID)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.ExecContext(context.Background(), fmt.Sprintf(`DROP POLICY portfolio_evidence_scope ON %s`, table)); err != nil {
				t.Error(err)
			}
		})
	}
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	session.db.SetMaxOpenConns(1)
	repository, _ := db.NewInvestigationRepository(session.db)
	t.Run("transfer pages and detail", func(t *testing.T) {
		items, _, err := repository.ListTransfers(ctx, testTenantID, testActorID, investigation.TransferFilter{Limit: 10})
		if err != nil || len(items) != 1 {
			t.Fatalf("scoped transfer list: count=%d err=%v", len(items), err)
		}
		detail, err := repository.GetTransfer(ctx, testTenantID, testActorID, transfer.Result.TransferID)
		if err != nil || len(detail.Postings) != 2 {
			t.Fatalf("scoped transfer detail: postings=%d err=%v", len(detail.Postings), err)
		}
	})
	t.Run("reconciliation pages and detail", func(t *testing.T) {
		items, _, err := repository.ListReconciliationRuns(ctx, testTenantID, testActorID, "", 10)
		if err != nil || len(items) != 1 {
			t.Fatalf("scoped reconciliation list: count=%d err=%v", len(items), err)
		}
		detail, err := repository.GetReconciliationRun(ctx, testTenantID, testActorID, run.ID)
		if err != nil || detail.ID != run.ID {
			t.Fatalf("scoped reconciliation detail: %v", err)
		}
	})
	t.Run("streamed evidence exports", func(t *testing.T) {
		exportRepository, _ := db.NewExportRepository(session.db)
		exporter, _ := appexports.NewService(exportRepository, 100, 10)
		var output bytes.Buffer
		result, err := exporter.StreamTransfers(ctx, testTenantID, testActorID, investigation.TransferFilter{}, 10, &output)
		if err != nil || result.Rows != 1 || !strings.Contains(output.String(), transfer.Result.TransferID) {
			t.Fatalf("scoped transfer export: rows=%d err=%v", result.Rows, err)
		}
		output.Reset()
		result, err = exporter.StreamReconciliation(ctx, testTenantID, testActorID, appexports.ReconciliationFilter{}, 10, &output)
		if err != nil || result.Rows != 1 || !strings.Contains(output.String(), run.ID) {
			t.Fatalf("scoped reconciliation export: rows=%d err=%v", result.Rows, err)
		}
	})
	t.Run("operations summary", func(t *testing.T) {
		operations, _ := db.NewOperationsRepository(session.db)
		facts, err := operations.Facts(ctx, testTenantID, testActorID)
		if err != nil || facts.PendingOutboxCount == 0 || facts.ReconciliationID != run.ID {
			t.Fatalf("scoped operations summary missing: %v", err)
		}
	})
	t.Run("staged reconciliation releases the only connection", func(t *testing.T) {
		commands := requireReconciliationCommandService(t, session.db)
		command := reconciliationCommand("tenant-staged-reconciliation-001")
		first, err := commands.Run(ctx, command)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := commands.Run(ctx, command)
		if err != nil || replay.Result.ID != first.Result.ID || !replay.Replayed {
			t.Fatalf("scoped staged reconciliation replay: %v", err)
		}
	})
}
