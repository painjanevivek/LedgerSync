package integration_test

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/openingimports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

const (
	requestOpeningImportFunction = `public.controlled_request_opening_import_v1(uuid,text,uuid,text,uuid[],bigint[],bytea,uuid,timestamptz)`
	approveOpeningImportFunction = `public.controlled_approve_opening_import_v1(uuid,text,uuid,bytea,uuid,timestamptz)`
	executeOpeningImportFunction = `public.controlled_execute_opening_import_v1(uuid,text,uuid,bytea,uuid,timestamptz)`
)

func TestControlledOpeningImportFunctionsUseFixedDefinerBoundary(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	for _, function := range []struct {
		name      string
		signature string
	}{
		{"controlled_request_opening_import_v1", requestOpeningImportFunction},
		{"controlled_approve_opening_import_v1", approveOpeningImportFunction},
		{"controlled_execute_opening_import_v1", executeOpeningImportFunction},
	} {
		var owner, searchPath string
		var securityDefiner, provisioningCanExecute, apiCanExecute, supportCanExecute, publicCanExecute bool
		err := database.QueryRowContext(context.Background(), `
SELECT owner.rolname,
       procedure.prosecdef,
       COALESCE((SELECT setting FROM unnest(procedure.proconfig) setting WHERE setting LIKE 'search_path=%'),''),
       has_function_privilege('ledgersync_provisioning',$1,'EXECUTE'),
       has_function_privilege('ledgersync_api',$1,'EXECUTE'),
       has_function_privilege('ledgersync_support_readonly',$1,'EXECUTE'),
       EXISTS(
         SELECT 1 FROM aclexplode(procedure.proacl) acl
         WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'
       )
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
JOIN pg_roles owner ON owner.oid=procedure.proowner
WHERE namespace.nspname='public' AND procedure.proname=$2`, function.signature, function.name).
			Scan(&owner, &securityDefiner, &searchPath, &provisioningCanExecute, &apiCanExecute, &supportCanExecute, &publicCanExecute)
		if err != nil {
			t.Fatal(err)
		}
		if owner != "ledgersync_migration_owner" || !securityDefiner || searchPath != "search_path=pg_catalog, public" || !provisioningCanExecute || apiCanExecute || supportCanExecute || publicCanExecute {
			t.Fatalf("unsafe %s metadata owner=%q definer=%t search_path=%q provisioning=%t api=%t support=%t public=%t", function.name, owner, securityDefiner, searchPath, provisioningCanExecute, apiCanExecute, supportCanExecute, publicCanExecute)
		}
	}
}

func TestControlledOpeningImportIsImmutableApprovedReconciledAndReplaySafe(t *testing.T) {
	service, database := requireAccountCommandService(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	const requester = "opening-import-requester"
	const approver = "opening-import-approver"
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance')`, testTenantID, requester, approver); err != nil {
		t.Fatal(err)
	}
	first, err := service.Create(ctx, openingImportAccountCommand("opening-import-account-001", "opening-import-one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, openingImportAccountCommand("opening-import-account-002", "opening-import-two"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := openingimports.Manifest{
		BatchID: "00000000-0000-4000-8000-000000008901", TenantID: testTenantID, Currency: "INR",
		Rows: []openingimports.Row{
			{AccountID: second.Result.AccountID, OpeningMinor: "2200"},
			{AccountID: first.Result.AccountID, OpeningMinor: "1100"},
		},
	}
	prepared, err := manifest.Validate(ctx, "INR")
	if err != nil {
		t.Fatal(err)
	}
	provisioning := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_provisioning")
	api := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")
	repository, err := db.NewOpeningImportRepository(provisioning.db, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	requested, err := repository.Request(ctx, prepared, requester, "00000000-0000-4000-8000-000000008902")
	if err != nil || requested.Replayed {
		t.Fatalf("request=%+v error=%v", requested, err)
	}
	replayed, err := repository.Request(ctx, prepared, requester, "00000000-0000-4000-8000-000000008902")
	if err != nil || !replayed.Replayed {
		t.Fatalf("request replay=%+v error=%v", replayed, err)
	}
	altered := manifest
	altered.Rows = append([]openingimports.Row(nil), manifest.Rows...)
	altered.Rows[0].OpeningMinor = "2201"
	alteredPrepared, err := altered.Validate(ctx, "INR")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Request(ctx, alteredPrepared, requester, "00000000-0000-4000-8000-000000008902"); !errors.Is(err, openingimports.ErrConflict) {
		t.Fatalf("altered request error=%v", err)
	}
	if _, err = repository.Approve(ctx, prepared, requester, "00000000-0000-4000-8000-000000008903"); !errors.Is(err, openingimports.ErrForbidden) {
		t.Fatalf("self approval error=%v", err)
	}
	if _, err = repository.Approve(ctx, alteredPrepared, approver, "00000000-0000-4000-8000-000000008903"); !errors.Is(err, openingimports.ErrInvalid) {
		t.Fatalf("altered approval error=%v", err)
	}
	approved, err := repository.Approve(ctx, prepared, approver, "00000000-0000-4000-8000-000000008903")
	if err != nil || approved.Replayed {
		t.Fatalf("approval=%+v error=%v", approved, err)
	}
	executed, err := repository.Execute(ctx, prepared, approver, "00000000-0000-4000-8000-000000008904")
	if err != nil || executed.Replayed {
		t.Fatalf("execution=%+v error=%v", executed, err)
	}
	executionReplay, err := repository.Execute(ctx, prepared, approver, "00000000-0000-4000-8000-000000008905")
	if err != nil || !executionReplay.Replayed {
		t.Fatalf("execution replay=%+v error=%v", executionReplay, err)
	}
	if countRows(t, database, `
SELECT count(*) FROM opening_import_rows row
JOIN account_opening_balances opening ON opening.account_id=row.account_id AND opening.opening_ledger_minor=row.opening_minor
JOIN account_balance_projections balance ON balance.account_id=row.account_id
  AND balance.available_minor=row.opening_minor AND balance.ledger_minor=row.opening_minor AND balance.balance_version=1
WHERE row.batch_id=$1`, prepared.BatchID) != 2 ||
		countRows(t, database, `SELECT count(*) FROM opening_import_executions WHERE batch_id=$1 AND row_count=2 AND total_minor=3300`, prepared.BatchID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM outbox_events WHERE payload->>'opening_import_id'=$1`, prepared.BatchID) != 2 ||
		countRows(t, database, `SELECT count(*) FROM audit_events WHERE target_id=$1 AND event_type IN ('opening_import.requested','opening_import.approved','opening_import.executed')`, prepared.BatchID) != 3 {
		t.Fatal("opening import evidence or financial state is incomplete")
	}
	var auditHash string
	if err = database.QueryRowContext(ctx, `SELECT sanitized_metadata->>'content_sha256' FROM audit_events WHERE target_id=$1 AND event_type='opening_import.executed'`, prepared.BatchID).Scan(&auditHash); err != nil || auditHash != hex.EncodeToString(prepared.ContentHash[:]) {
		t.Fatalf("execution audit hash=%q error=%v", auditHash, err)
	}
	if _, err = database.ExecContext(ctx, `UPDATE opening_import_rows SET opening_minor=opening_minor+1 WHERE batch_id=$1`, prepared.BatchID); sqlState(err) != "55000" {
		t.Fatalf("manifest alteration SQLSTATE=%s error=%v, want 55000", sqlState(err), err)
	}
	if _, err = api.db.ExecContext(ctx, `SELECT * FROM public.controlled_execute_opening_import_v1($1,$2,$3,$4,$5,$6)`, prepared.TenantID, approver, prepared.BatchID, prepared.ContentHash[:], "00000000-0000-4000-8000-000000008906", now); sqlState(err) != "42501" {
		t.Fatalf("API execution SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}
	if _, err = provisioning.db.ExecContext(ctx, `SELECT * FROM public.controlled_execute_opening_import_v1($1,$2,$3,$4,$5,$6)`, prepared.TenantID, "spoofed-finance", prepared.BatchID, prepared.ContentHash[:], "00000000-0000-4000-8000-000000008906", now); sqlState(err) != "42501" {
		t.Fatalf("spoofed execution SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}

	tx, err := provisioning.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`CREATE TEMP TABLE opening_import_batches(id uuid); SET LOCAL search_path=pg_temp,public`); err != nil {
		t.Fatal(err)
	}
	var conflicted bool
	if err = tx.QueryRow(`SELECT replayed,conflicted FROM public.controlled_execute_opening_import_v1($1,$2,$3,$4,$5,$6)`, prepared.TenantID, approver, prepared.BatchID, prepared.ContentHash[:], "00000000-0000-4000-8000-000000008907", now).Scan(&replayed.Replayed, &conflicted); err != nil || !replayed.Replayed || conflicted {
		t.Fatalf("fixed search path replay=%t conflicted=%t error=%v", replayed.Replayed, conflicted, err)
	}
}

func TestControlledOpeningImportFailureIsAtomicWhenAccountStateChanges(t *testing.T) {
	service, database := requireAccountCommandService(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)
	const requester = "opening-import-requester"
	const approver = "opening-import-approver"
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance')`, testTenantID, requester, approver); err != nil {
		t.Fatal(err)
	}
	first, err := service.Create(ctx, openingImportAccountCommand("opening-import-atomic-001", "opening-import-atomic-one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, openingImportAccountCommand("opening-import-atomic-002", "opening-import-atomic-two"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := openingimports.Manifest{
		BatchID: "00000000-0000-4000-8000-000000008921", TenantID: testTenantID, Currency: "INR",
		Rows: []openingimports.Row{{AccountID: first.Result.AccountID, OpeningMinor: "100"}, {AccountID: second.Result.AccountID, OpeningMinor: "200"}},
	}
	prepared, err := manifest.Validate(ctx, "INR")
	if err != nil {
		t.Fatal(err)
	}
	provisioning := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_provisioning")
	repository, _ := db.NewOpeningImportRepository(provisioning.db, func() time.Time { return now })
	if _, err = repository.Request(ctx, prepared, requester, "00000000-0000-4000-8000-000000008922"); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Approve(ctx, prepared, approver, "00000000-0000-4000-8000-000000008923"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `UPDATE account_balance_projections SET available_minor=1,ledger_minor=1 WHERE account_id=$1`, first.Result.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Execute(ctx, prepared, approver, "00000000-0000-4000-8000-000000008924"); !errors.Is(err, openingimports.ErrConflict) {
		t.Fatalf("changed-state execution error=%v", err)
	}
	if countRows(t, database, `SELECT count(*) FROM opening_import_executions WHERE batch_id=$1`, prepared.BatchID) != 0 ||
		countRows(t, database, `SELECT count(*) FROM account_opening_balances WHERE account_id=$1 AND opening_ledger_minor=0`, second.Result.AccountID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM account_balance_projections WHERE account_id=$1 AND available_minor=0 AND ledger_minor=0`, second.Result.AccountID) != 1 {
		t.Fatal("failed opening import partially mutated unaffected accounts")
	}
}

func openingImportAccountCommand(key, reference string) accounts.CreateAccountCommand {
	command := createAccountCommand(key, reference)
	command.CorrelationID = "00000000-0000-4000-8000-000000008990"
	return command
}
