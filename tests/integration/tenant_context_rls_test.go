package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	platformdb "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantContextExpandScopesReadsRejectsMismatchesAndDoesNotLeak(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	ctx := context.Background()
	const otherTenant = "00000000-0000-4000-8000-000000009901"
	const otherAccount = "00000000-0000-4000-8000-000000009902"
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants(id,external_reference) VALUES($1,'tenant-context-other')`, otherTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference) VALUES($1,$2,'USD','active','Other tenant','operating','tenant-context-other')`, otherAccount, otherTenant); err != nil {
		t.Fatal(err)
	}

	api := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")
	api.db.SetMaxOpenConns(1)
	api.db.SetMaxIdleConns(1)

	var compatibleCount int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM accounts`).Scan(&compatibleCount); err != nil {
		t.Fatal(err)
	}
	if compatibleCount != 3 {
		t.Fatalf("expand-mode missing context count=%d, want 3", compatibleCount)
	}

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = platformdb.SetLocalTenantContext(ctx, tx, testTenantID); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	var scopedCount, foreignCount int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM accounts`).Scan(&scopedCount); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM accounts WHERE id=$1`, otherAccount).Scan(&foreignCount); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	_, mismatchErr := tx.ExecContext(ctx, `SELECT public.controlled_append_audit_event_v1($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10)`,
		"00000000-0000-4000-8000-000000009903", otherTenant, testActorID, "tenant.context.probe", "tenant", otherTenant,
		"denied", "00000000-0000-4000-8000-000000009904", []byte(`{}`), time.Now().UTC())
	_ = tx.Rollback()
	if scopedCount != 2 || foreignCount != 0 {
		t.Fatalf("tenant-scoped account visibility scoped=%d foreign=%d", scopedCount, foreignCount)
	}
	if sqlState(mismatchErr) != "42501" {
		t.Fatalf("mismatched controlled write SQLSTATE=%s error=%v, want 42501", sqlState(mismatchErr), mismatchErr)
	}

	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM accounts`).Scan(&compatibleCount); err != nil {
		t.Fatal(err)
	}
	if compatibleCount != 3 {
		t.Fatalf("pooled connection leaked transaction-local tenant context: count=%d, want 3", compatibleCount)
	}

	invalid, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = platformdb.SetLocalTenantContext(ctx, invalid, "not-a-tenant"); !errors.Is(err, platformdb.ErrInvalidTenantContext) {
		_ = invalid.Rollback()
		t.Fatalf("invalid tenant context error=%v", err)
	}
	_ = invalid.Rollback()
}

func TestTenantContextExpandMigrationMetadata(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	var enabled, forced, policies, triggers int
	if err := database.QueryRow(`
SELECT count(*) FILTER (WHERE class.relrowsecurity),count(*) FILTER (WHERE class.relforcerowsecurity)
FROM pg_class class JOIN pg_namespace namespace ON namespace.oid=class.relnamespace
WHERE namespace.nspname='public' AND class.relname=ANY($1::text[])`, []string{
		"accounts", "account_owners", "account_credit_permissions", "account_balance_projections", "account_opening_balances",
		"transfers", "transfer_velocity_events", "transfer_velocity_totals", "funding_events", "funding_velocity_events",
		"transfer_corrections", "approval_records", "journal_transactions", "ledger_postings", "audit_events",
		"opening_import_batches", "opening_import_rows", "opening_import_approvals", "opening_import_executions",
	}).Scan(&enabled, &forced); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM pg_policies WHERE schemaname='public' AND policyname='tenant_context_expand'`).Scan(&policies); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM pg_trigger WHERE tgname='enforce_tenant_context' AND NOT tgisinternal`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if enabled != 19 || forced != 0 || policies != 19 || triggers != 17 {
		t.Fatalf("tenant expand metadata enabled=%d forced=%d policies=%d triggers=%d", enabled, forced, policies, triggers)
	}
}
