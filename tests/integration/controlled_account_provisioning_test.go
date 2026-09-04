package integration_test

import (
	"context"
	"testing"
	"time"
)

const controlledAccountFunction = `public.controlled_provision_account_v1(uuid,text,uuid,text,text,text,text,text[],text[],text[],uuid,timestamptz)`

func TestControlledAccountFunctionUsesFixedDefinerBoundary(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)

	var owner, searchPath string
	var securityDefiner, apiCanExecute, provisioningCanExecute, supportCanExecute, ownerCanWriteAccount, publicCanExecute bool
	err := database.QueryRowContext(context.Background(), `
SELECT owner.rolname,
       procedure.prosecdef,
       COALESCE((SELECT setting FROM unnest(procedure.proconfig) setting WHERE setting LIKE 'search_path=%'),''),
       has_function_privilege('ledgersync_api',$1,'EXECUTE'),
       has_function_privilege('ledgersync_provisioning',$1,'EXECUTE'),
       has_function_privilege('ledgersync_support_readonly',$1,'EXECUTE'),
       has_table_privilege('ledgersync_migration_owner','public.accounts','INSERT')
         AND has_table_privilege('ledgersync_migration_owner','public.account_opening_balances','INSERT'),
       EXISTS(
         SELECT 1 FROM aclexplode(procedure.proacl) acl
         WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'
       )
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
JOIN pg_roles owner ON owner.oid=procedure.proowner
WHERE namespace.nspname='public' AND procedure.proname='controlled_provision_account_v1'`, controlledAccountFunction).
		Scan(&owner, &securityDefiner, &searchPath, &apiCanExecute, &provisioningCanExecute, &supportCanExecute, &ownerCanWriteAccount, &publicCanExecute)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ledgersync_migration_owner" || !securityDefiner || searchPath != "search_path=pg_catalog, public" || !apiCanExecute || !provisioningCanExecute || supportCanExecute || !ownerCanWriteAccount || publicCanExecute {
		t.Fatalf("unsafe controlled account metadata owner=%q definer=%t search_path=%q api=%t provisioning=%t support=%t owner_write=%t public=%t", owner, securityDefiner, searchPath, apiCanExecute, provisioningCanExecute, supportCanExecute, ownerCanWriteAccount, publicCanExecute)
	}
}

func TestControlledAccountFunctionForcesZeroAndRejectsSpoofing(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	api := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")
	provisioning := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_provisioning")
	support := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_support_readonly")
	const accountID = "00000000-0000-4000-8000-000000008701"
	const provisioningAccountID = "00000000-0000-4000-8000-000000008702"
	const correlationID = "00000000-0000-4000-8000-000000008703"
	args := []any{
		testTenantID, testActorID, accountID, "USD", "Controlled account", "operating", "controlled-account-001",
		[]string{testActorID}, []string{testActorID}, []string{testActorID}, correlationID, now,
	}
	var replayed, conflicted bool
	if err := api.db.QueryRowContext(ctx, `SELECT replayed,conflicted FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, args...).Scan(&replayed, &conflicted); err != nil || replayed || conflicted {
		t.Fatalf("API controlled account replayed=%t conflicted=%t error=%v", replayed, conflicted, err)
	}
	if countRows(t, database, `
SELECT count(*)
FROM accounts account
JOIN account_balance_projections balance ON balance.account_id=account.id
JOIN account_opening_balances opening ON opening.account_id=account.id
WHERE account.id=$1 AND account.tenant_id=$2 AND balance.available_minor=0
  AND balance.ledger_minor=0 AND opening.opening_ledger_minor=0`, accountID, testTenantID) != 1 {
		t.Fatal("controlled account did not create an exact zero opening state")
	}
	if err := api.db.QueryRowContext(ctx, `SELECT replayed,conflicted FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, args...).Scan(&replayed, &conflicted); err != nil || !replayed || conflicted {
		t.Fatalf("controlled account replay replayed=%t conflicted=%t error=%v", replayed, conflicted, err)
	}
	changed := append([]any(nil), args...)
	changed[4] = "Changed account"
	if err := api.db.QueryRowContext(ctx, `SELECT replayed,conflicted FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, changed...).Scan(&replayed, &conflicted); err != nil || replayed || !conflicted {
		t.Fatalf("changed duplicate replayed=%t conflicted=%t error=%v", replayed, conflicted, err)
	}
	spoofed := append([]any(nil), args...)
	spoofed[2] = "00000000-0000-4000-8000-000000008704"
	spoofed[1] = "spoofed-operator"
	if _, err := api.db.ExecContext(ctx, `SELECT * FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, spoofed...); sqlState(err) != "42501" {
		t.Fatalf("spoofed actor SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}
	if _, err := support.db.ExecContext(ctx, `SELECT * FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, args...); sqlState(err) != "42501" {
		t.Fatalf("support execution SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}

	provisioningArgs := append([]any(nil), args...)
	provisioningArgs[1] = "platform-provisioner"
	provisioningArgs[2] = provisioningAccountID
	provisioningArgs[4] = "Provisioning account"
	provisioningArgs[6] = "controlled-account-002"
	if err := provisioning.db.QueryRowContext(ctx, `SELECT replayed,conflicted FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, provisioningArgs...).Scan(&replayed, &conflicted); err != nil || replayed || conflicted {
		t.Fatalf("provisioning controlled account replayed=%t conflicted=%t error=%v", replayed, conflicted, err)
	}

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`CREATE TEMP TABLE accounts(id uuid); SET LOCAL search_path=pg_temp,public`); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(`SELECT replayed,conflicted FROM public.controlled_provision_account_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, args...).Scan(&replayed, &conflicted); err != nil || !replayed || conflicted {
		t.Fatalf("fixed search path replayed=%t conflicted=%t error=%v", replayed, conflicted, err)
	}
}
