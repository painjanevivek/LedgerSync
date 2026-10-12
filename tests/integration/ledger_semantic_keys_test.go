package integration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLedgerSemanticKeysSupportOldAndExpandedWriters(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	ctx := context.Background()

	expanded, err := service.Submit(ctx, transferCommand(t, "semantic-expanded-writer-0001", "5.00"))
	if err != nil {
		t.Fatalf("submit expanded-writer transfer: %v", err)
	}
	var sourceType, sourceID, journalTenant string
	var postingCount int
	if err := database.QueryRowContext(ctx, `
SELECT journal.source_type,journal.source_id::text,journal.tenant_id::text,
       count(posting.id) FILTER (WHERE posting.tenant_id=journal.tenant_id)
FROM journal_transactions AS journal
JOIN ledger_postings AS posting
  ON posting.journal_transaction_id=journal.id AND posting.tenant_id=journal.tenant_id
WHERE journal.transfer_id=$1
GROUP BY journal.id`, expanded.Result.TransferID).Scan(&sourceType, &sourceID, &journalTenant, &postingCount); err != nil {
		t.Fatalf("read expanded semantic keys: %v", err)
	}
	if sourceType != "transfer" || sourceID != expanded.Result.TransferID || journalTenant != testTenantID || postingCount != 2 {
		t.Fatalf("expanded semantic keys type=%q source=%q tenant=%q postings=%d", sourceType, sourceID, journalTenant, postingCount)
	}

	const oldTransferID = "00000000-0000-4000-8000-000000000911"
	const oldJournalID = "00000000-0000-4000-8000-000000000912"
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,created_at,policy_version)
VALUES($1,$2,$3,$4,$5,1,'USD','pending',$6,1)`, oldTransferID, testTenantID, testActorID, testSourceID, testDestinationID, time.Now().UTC()); err != nil {
		t.Fatalf("insert old-writer transfer: %v", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO journal_transactions(id,tenant_id,transfer_id,occurred_at) VALUES($1,$2,$3,$4)`, oldJournalID, testTenantID, oldTransferID, time.Now().UTC()); err != nil {
		t.Fatalf("insert old-writer journal: %v", err)
	}
	if _, err = tx.ExecContext(ctx, `
INSERT INTO ledger_postings(id,journal_transaction_id,account_id,direction,amount_minor,currency,occurred_at) VALUES
('00000000-0000-4000-8000-000000000913',$1,$2,'debit',1,'USD',$4),
('00000000-0000-4000-8000-000000000914',$1,$3,'credit',1,'USD',$4)`, oldJournalID, testSourceID, testDestinationID, time.Now().UTC()); err != nil {
		t.Fatalf("insert old-writer postings: %v", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE transfers SET status='posted',journal_transaction_id=$2,completed_at=$3 WHERE id=$1`, oldTransferID, oldJournalID, time.Now().UTC()); err != nil {
		t.Fatalf("complete old-writer transfer: %v", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE account_balance_projections SET available_minor=available_minor-1,ledger_minor=ledger_minor-1,balance_version=balance_version+1 WHERE account_id=$1`, testSourceID); err != nil {
		t.Fatalf("apply old-writer debit projection: %v", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE account_balance_projections SET available_minor=available_minor+1,ledger_minor=ledger_minor+1,balance_version=balance_version+1 WHERE account_id=$1`, testDestinationID); err != nil {
		t.Fatalf("apply old-writer credit projection: %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit old-writer ledger shape: %v", err)
	}
	if err := database.QueryRowContext(ctx, `
SELECT journal.source_type,journal.source_id::text,count(posting.id) FILTER (WHERE posting.tenant_id=$2)
FROM journal_transactions AS journal
JOIN ledger_postings AS posting ON posting.journal_transaction_id=journal.id
WHERE journal.id=$1
GROUP BY journal.id`, oldJournalID, testTenantID).Scan(&sourceType, &sourceID, &postingCount); err != nil {
		t.Fatalf("read hydrated old-writer keys: %v", err)
	}
	if sourceType != "transfer" || sourceID != oldTransferID || postingCount != 2 {
		t.Fatalf("old-writer hydration type=%q source=%q postings=%d", sourceType, sourceID, postingCount)
	}
	assertNoLedgerSemanticKeyMismatches(t, database)
}

func TestLedgerSemanticKeysRejectNewTenantSubstitutionAndReportHistoricalMismatch(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	const otherTenantID = "00000000-0000-4000-8000-000000000921"
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants(id,external_reference) VALUES($1,'semantic-other-tenant')`, otherTenantID); err != nil {
		t.Fatal(err)
	}
	const transferID = "00000000-0000-4000-8000-000000000922"
	if _, err := database.ExecContext(ctx, `
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,created_at,policy_version)
VALUES($1,$2,$3,$4,$5,1,'USD','pending',$6,1)`, transferID, testTenantID, testActorID, testSourceID, testDestinationID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at)
VALUES('00000000-0000-4000-8000-000000000923',$1,$2,'transfer',$2,$3)`, otherTenantID, transferID, time.Now().UTC()); sqlState(err) != "23503" {
		t.Fatalf("journal tenant substitution SQLSTATE=%s error=%v, want 23503", sqlState(err), err)
	}

	posted, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = posted.Rollback() }()
	const validJournalID = "00000000-0000-4000-8000-000000000924"
	if _, err = posted.ExecContext(ctx, `INSERT INTO journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at) VALUES($1,$2,$3,'transfer',$3,$4)`, validJournalID, testTenantID, transferID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = posted.ExecContext(ctx, `
INSERT INTO ledger_postings(id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at)
VALUES('00000000-0000-4000-8000-000000000925',$1,$2,$3,'debit',1,'USD',$4)`, validJournalID, otherTenantID, testSourceID, time.Now().UTC()); sqlState(err) != "23503" {
		t.Fatalf("posting tenant substitution SQLSTATE=%s error=%v, want 23503", sqlState(err), err)
	}
	_ = posted.Rollback()

	historical, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = historical.Rollback() }()
	if _, err = historical.ExecContext(ctx, `SET LOCAL session_replication_role='replica'`); err != nil {
		t.Fatalf("enable historical-shape fixture: %v", err)
	}
	// Simulate a row written before migration 000035 validated and enforced the
	// source identity check. The DDL and fixture remain isolated in this rolled-
	// back transaction; new writes above still prove the live constraint path.
	if _, err = historical.ExecContext(ctx, `ALTER TABLE journal_transactions DROP CONSTRAINT journal_source_matches_command_check`); err != nil {
		t.Fatalf("simulate pre-enforcement schema: %v", err)
	}
	if _, err = historical.ExecContext(ctx, `
INSERT INTO journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at)
VALUES('00000000-0000-4000-8000-000000000926',$1,$2,'transfer','00000000-0000-4000-8000-000000000927',$3)`, otherTenantID, transferID, time.Now().UTC()); err != nil {
		t.Fatalf("insert historical mismatch fixture: %v", err)
	}
	var sourceMismatches, tenantMismatches int
	if err = historical.QueryRowContext(ctx, `SELECT journal_source_mismatch_count,transfer_tenant_mismatch_count FROM ledger_semantic_key_validation`).Scan(&sourceMismatches, &tenantMismatches); err != nil {
		t.Fatal(err)
	}
	if sourceMismatches != 1 || tenantMismatches != 1 {
		t.Fatalf("historical mismatch report source=%d tenant=%d, want 1/1", sourceMismatches, tenantMismatches)
	}
}

func TestLedgerSemanticKeyDownMigrationRefusesUnrepresentableEvidence(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	_, err := service.Submit(ctx, transferCommand(t, "semantic-down-contract-0001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	downSQL := readLedgerSemanticDownMigration(t)
	validationDownSQL := readMigrationFile(t, "000035_ledger_semantic_validation.down.sql")

	unsafe, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unsafe.Rollback() }()
	if _, err = unsafe.ExecContext(ctx, `ALTER TABLE journal_transactions DROP CONSTRAINT journal_source_matches_command_check`); err != nil {
		t.Fatal(err)
	}
	if _, err = unsafe.ExecContext(ctx, `
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,created_at,policy_version)
VALUES('00000000-0000-4000-8000-000000000931',$1,$2,$3,$4,1,'USD','pending',$5,1)`, testTenantID, testActorID, testSourceID, testDestinationID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = unsafe.ExecContext(ctx, `
INSERT INTO journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at)
VALUES('00000000-0000-4000-8000-000000000932',$1,'00000000-0000-4000-8000-000000000931','transfer','00000000-0000-4000-8000-000000000933',$2)`, testTenantID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = unsafe.ExecContext(ctx, downSQL); sqlState(err) != "55000" {
		t.Fatalf("unsafe down migration SQLSTATE=%s error=%v, want 55000", sqlState(err), err)
	}
	_ = unsafe.Rollback()

	safe, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = safe.Rollback() }()
	if _, err = safe.ExecContext(ctx, `SET LOCAL ledgersync.semantic_validation_rollback_reason='incident commander and ledger owner approved test rollback'`); err != nil {
		t.Fatal(err)
	}
	if _, err = safe.ExecContext(ctx, validationDownSQL); err != nil {
		t.Fatalf("approved semantic-validation down migration: %v", err)
	}
	var rollbackMarkers int
	if err = safe.QueryRowContext(ctx, `SELECT count(*) FROM ledger_semantic_control_events WHERE action='semantic_validation_disabled'`).Scan(&rollbackMarkers); err != nil {
		t.Fatal(err)
	}
	if rollbackMarkers != 1 {
		t.Fatalf("semantic-validation down migration markers=%d, want 1", rollbackMarkers)
	}
	if _, err = safe.ExecContext(ctx, downSQL); err != nil {
		t.Fatalf("representable down migration: %v", err)
	}
	var remainingColumns int
	if err = safe.QueryRowContext(ctx, `
SELECT count(*) FROM information_schema.columns
WHERE table_schema='public'
  AND ((table_name='journal_transactions' AND column_name IN ('source_type','source_id'))
    OR (table_name='ledger_postings' AND column_name='tenant_id'))`).Scan(&remainingColumns); err != nil {
		t.Fatal(err)
	}
	if remainingColumns != 0 {
		t.Fatalf("down migration left %d expanded columns", remainingColumns)
	}
}

func seedProductionLikeHistoricalLedger(t *testing.T, database *sql.DB, tenantID, sourceAccountID, destinationAccountID string, occurredAt time.Time, journalCount int) {
	t.Helper()
	tx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,journal_transaction_id,created_at,completed_at)
SELECT md5('semantic-transfer-'||series)::uuid,$1,'upgrade-operator',
       CASE WHEN series%2=1 THEN $2::uuid ELSE $3::uuid END,
       CASE WHEN series%2=1 THEN $3::uuid ELSE $2::uuid END,
       1,'INR','posted',md5('semantic-journal-'||series)::uuid,
       $4::timestamptz+(series||' milliseconds')::interval,
       $4::timestamptz+(series||' milliseconds')::interval
FROM generate_series(1,$5) AS series`, tenantID, sourceAccountID, destinationAccountID, occurredAt, journalCount); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO journal_transactions(id,tenant_id,transfer_id,occurred_at)
SELECT md5('semantic-journal-'||series)::uuid,$1,md5('semantic-transfer-'||series)::uuid,
       $2::timestamptz+(series||' milliseconds')::interval
FROM generate_series(1,$3) AS series`, tenantID, occurredAt, journalCount); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO ledger_postings(id,journal_transaction_id,account_id,direction,amount_minor,currency,occurred_at)
SELECT md5(kind||'-'||series)::uuid,md5('semantic-journal-'||series)::uuid,
       CASE
         WHEN (series%2=1 AND kind='semantic-debit') OR (series%2=0 AND kind='semantic-credit') THEN $1::uuid
         ELSE $2::uuid
       END,
       CASE WHEN kind='semantic-debit' THEN 'debit' ELSE 'credit' END,
       1,'INR',$3::timestamptz+(series||' milliseconds')::interval
FROM generate_series(1,$4) AS series
CROSS JOIN (VALUES('semantic-debit'),('semantic-credit')) AS kinds(kind)`, sourceAccountID, destinationAccountID, occurredAt, journalCount); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit production-like historical ledger shape: %v", err)
	}
}

func assertNoLedgerSemanticKeyMismatches(t *testing.T, database interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) {
	t.Helper()
	var unbackfilledJournals, unbackfilledPostings, sourceMismatch, transferMismatch int
	var fundingMismatch, journalTenantMismatch, accountTenantMismatch, duplicateSources int
	if err := database.QueryRowContext(context.Background(), `
SELECT unbackfilled_journal_count,unbackfilled_posting_count,journal_source_mismatch_count,
       transfer_tenant_mismatch_count,funding_tenant_mismatch_count,
       posting_journal_tenant_mismatch_count,posting_account_tenant_mismatch_count,
       duplicate_journal_source_count
FROM ledger_semantic_key_validation`).Scan(
		&unbackfilledJournals, &unbackfilledPostings, &sourceMismatch, &transferMismatch,
		&fundingMismatch, &journalTenantMismatch, &accountTenantMismatch, &duplicateSources,
	); err != nil {
		t.Fatalf("read ledger semantic validation: %v", err)
	}
	if unbackfilledJournals+unbackfilledPostings+sourceMismatch+transferMismatch+fundingMismatch+journalTenantMismatch+accountTenantMismatch+duplicateSources != 0 {
		t.Fatalf("ledger semantic validation reported mismatches: unbackfilled_journals=%d unbackfilled_postings=%d source=%d transfer_tenant=%d funding_tenant=%d posting_journal=%d posting_account=%d duplicates=%d", unbackfilledJournals, unbackfilledPostings, sourceMismatch, transferMismatch, fundingMismatch, journalTenantMismatch, accountTenantMismatch, duplicateSources)
	}
}

func readLedgerSemanticDownMigration(t *testing.T) string {
	return readMigrationFile(t, "000034_ledger_semantic_keys_expand.down.sql")
}

func readMigrationFile(t *testing.T, name string) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate ledger semantic migration")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "..", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
