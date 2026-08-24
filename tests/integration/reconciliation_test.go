package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestPostedTransferProjectionReconcilesWithImmutableLedger(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	result, err := service.Submit(context.Background(), transferCommand(t, "reconciliation-transfer-key-01", "25.00"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Result.Status != "posted" {
		t.Fatalf("status = %q", result.Result.Status)
	}
	const reconciliation = `
SELECT p.account_id, p.ledger_minor,
       COALESCE(SUM(CASE WHEN l.direction = 'credit' THEN l.amount_minor ELSE -l.amount_minor END), 0) AS recomputed
FROM account_balance_projections p
LEFT JOIN ledger_postings l ON l.account_id = p.account_id
WHERE p.account_id IN ($1, $2)
GROUP BY p.account_id, p.ledger_minor
ORDER BY p.account_id`
	rows, err := database.Query(reconciliation, testSourceID, testDestinationID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var accountID string
		var projection, recomputed int64
		if err := rows.Scan(&accountID, &projection, &recomputed); err != nil {
			t.Fatal(err)
		}
		// Fixtures start with an opening balance that predates this transfer, so
		// reconcile the delta from this journal rather than the fixture seed.
		if accountID == testSourceID && projection != 7500 {
			t.Fatalf("source projection = %d", projection)
		}
		if accountID == testDestinationID && projection != 4500 {
			t.Fatalf("destination projection = %d", projection)
		}
		if accountID == testSourceID && recomputed != -2500 {
			t.Fatalf("source ledger delta = %d", recomputed)
		}
		if accountID == testDestinationID && recomputed != 2500 {
			t.Fatalf("destination ledger delta = %d", recomputed)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestOperatorReconciliationMatchedMismatchAndIdempotentReplay(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	service := requireReconciliationCommandService(t, database)
	original, err := service.Run(context.Background(), reconciliationCommand("reconciliation-run-key-0001"))
	if err != nil {
		t.Fatal(err)
	}
	if original.Result.Status != reconciliation.StatusMatched || original.Replayed {
		t.Fatalf("original = %#v", original)
	}
	replay, err := service.Run(context.Background(), reconciliationCommand("reconciliation-run-key-0001"))
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.Result.ID != original.Result.ID {
		t.Fatalf("replay = %#v, original = %#v", replay, original)
	}
	if countRows(t, database, `SELECT count(*) FROM reconciliation_runs WHERE tenant_id=$1`, testTenantID) != 1 || countRows(t, database, `SELECT count(*) FROM audit_events WHERE tenant_id=$1 AND event_type='reconciliation.requested'`, testTenantID) != 1 || countRows(t, database, `SELECT count(*) FROM audit_events WHERE tenant_id=$1 AND event_type='reconciliation.completed'`, testTenantID) != 1 {
		t.Fatal("replay duplicated durable reconciliation evidence")
	}

	if _, err := database.Exec(`UPDATE account_balance_projections SET ledger_minor=ledger_minor+1 WHERE account_id=$1`, testSourceID); err != nil {
		t.Fatal(err)
	}
	mismatch, err := service.Run(context.Background(), reconciliationCommand("reconciliation-run-key-0002"))
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.Result.Status != reconciliation.StatusMismatch || mismatch.Result.MismatchCount == 0 {
		t.Fatalf("seeded mismatch was not detected: %#v", mismatch)
	}
}

func TestOperatorReconciliationDifferentKeyAlreadyRunningIsStable(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	service := requireReconciliationCommandService(t, database)
	lockTx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lockTx.Rollback() }()
	if _, err := lockTx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended($1, 824631))`, testTenantID); err != nil {
		t.Fatal(err)
	}
	const activeRunID = "00000000-0000-4000-8000-000000000077"
	if _, err := database.Exec(`INSERT INTO reconciliation_run_commands(tenant_id,run_id,actor_subject_id,idempotency_key,correlation_id,lease_expires_at,requested_at) VALUES($1,$2,'other-operator','reconciliation-active-key-0001','00000000-0000-4000-8000-000000000076',now()+interval '5 minutes',now())`, testTenantID, activeRunID); err != nil {
		t.Fatal(err)
	}
	const secondTenant = "00000000-0000-0000-0000-000000000002"
	if _, err := database.Exec(`INSERT INTO tenants(id,external_reference) VALUES ($1,'reconciliation-second-tenant')`, secondTenant); err != nil {
		t.Fatal(err)
	}
	other := reconciliationCommand("reconciliation-run-key-other-tenant")
	other.TenantID = secondTenant
	other.CorrelationID = "00000000-0000-4000-8000-000000000098"
	otherResult, err := service.Run(context.Background(), other)
	if err != nil || otherResult.Denial != "" {
		t.Fatalf("different tenant was incorrectly excluded: result=%#v error=%v", otherResult, err)
	}

	original, err := service.Run(context.Background(), reconciliationCommand("reconciliation-run-key-locked"))
	if err != nil {
		t.Fatal(err)
	}
	if original.Denial != "already_running" || original.Replayed || original.ActiveRunID != activeRunID {
		t.Fatalf("original denial = %#v", original)
	}
	replay, err := service.Run(context.Background(), reconciliationCommand("reconciliation-run-key-locked"))
	if err != nil {
		t.Fatal(err)
	}
	if replay.Denial != "already_running" || !replay.Replayed {
		t.Fatalf("replayed denial = %#v", replay)
	}
	if countRows(t, database, `SELECT count(*) FROM reconciliation_runs WHERE tenant_id=$1`, testTenantID) != 0 || countRows(t, database, `SELECT count(*) FROM audit_events WHERE tenant_id=$1 AND event_type='reconciliation.command_denied'`, testTenantID) != 1 {
		t.Fatal("already-running denial created a run or duplicated its audit")
	}
	if err := lockTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`DELETE FROM reconciliation_run_commands WHERE tenant_id=$1`, testTenantID); err != nil {
		t.Fatal(err)
	}
	afterRelease, err := service.Run(context.Background(), reconciliationCommand("reconciliation-run-key-after-release"))
	if err != nil || afterRelease.Denial != "" || afterRelease.Result.ID == "" {
		t.Fatalf("tenant lock was not released: result=%#v error=%v", afterRelease, err)
	}
}

func TestOperatorReconciliationAuditFailureRollsBackRunAndIdempotency(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	service := requireReconciliationCommandService(t, database)
	if _, err := database.Exec(`CREATE OR REPLACE FUNCTION ledgersync_test_reconciliation_audit_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type='reconciliation.requested' THEN RAISE EXCEPTION 'forced audit failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TRIGGER ledgersync_test_reconciliation_audit_failure BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION ledgersync_test_reconciliation_audit_failure()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DROP TRIGGER IF EXISTS ledgersync_test_reconciliation_audit_failure ON audit_events`)
		_, _ = database.Exec(`DROP FUNCTION IF EXISTS ledgersync_test_reconciliation_audit_failure()`)
	})
	command := reconciliationCommand("reconciliation-run-key-audit-fail")
	if _, err := service.Run(context.Background(), command); err == nil {
		t.Fatal("forced audit failure was reported as success")
	}
	if countRows(t, database, `SELECT count(*) FROM reconciliation_runs WHERE tenant_id=$1`, testTenantID) != 0 || countRows(t, database, `SELECT count(*) FROM idempotency_requests WHERE tenant_id=$1 AND operation=$2 AND idempotency_key=$3`, testTenantID, reconciliation.RunOperation, command.IdempotencyKey) != 0 {
		t.Fatal("failed request audit did not roll back command state")
	}
}

func requireReconciliationCommandService(t *testing.T, database *sql.DB) *reconciliation.CommandService {
	t.Helper()
	repository, err := db.NewReconciliationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := reconciliation.NewCommandService(repository, func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func reconciliationCommand(key string) reconciliation.RunCommand {
	return reconciliation.RunCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID,
		CorrelationID: "00000000-0000-4000-8000-000000000099", IdempotencyKey: key,
	}
}
