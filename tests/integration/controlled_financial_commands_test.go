package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	apptransfers "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
)

const controlledTransferFunction = `public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid)`

func TestControlledTransferFunctionUsesFixedDefinerBoundary(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)

	var owner, searchPath string
	var securityDefiner, apiCanExecute, supportCanExecute, publicCanExecute bool
	err := database.QueryRowContext(context.Background(), `
SELECT owner.rolname,
       procedure.prosecdef,
       COALESCE((SELECT setting FROM unnest(procedure.proconfig) setting WHERE setting LIKE 'search_path=%'),''),
	       has_function_privilege('ledgersync_api',$1,'EXECUTE'),
	       has_function_privilege('ledgersync_support_readonly',$1,'EXECUTE'),
	       EXISTS(
	         SELECT 1 FROM aclexplode(procedure.proacl) acl
	         WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'
	       )
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
JOIN pg_roles owner ON owner.oid=procedure.proowner
WHERE namespace.nspname='public' AND procedure.proname='controlled_submit_transfer_v1'`, controlledTransferFunction).
		Scan(&owner, &securityDefiner, &searchPath, &apiCanExecute, &supportCanExecute, &publicCanExecute)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ledgersync_migration_owner" || !securityDefiner || searchPath != "search_path=pg_catalog, public" || !apiCanExecute || supportCanExecute || publicCanExecute {
		t.Fatalf("unsafe controlled transfer metadata owner=%q definer=%t search_path=%q api=%t support=%t public=%t", owner, securityDefiner, searchPath, apiCanExecute, supportCanExecute, publicCanExecute)
	}
}

func TestControlledTransferFunctionCommitsAndReplaysAsAPIWorkload(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	session := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")

	command := transferCommand(t, "controlled-transfer-0001", "1.00")
	fingerprint, err := apptransfers.Fingerprint(apptransfers.IdempotencyRequest{
		TenantID: command.TenantID, ActorSubjectID: command.ActorSubjectID, Operation: "transfers.create.v1", Key: command.IdempotencyKey,
		DebitAccountID: command.DebitAccountID, CreditAccountID: command.CreditAccountID, Amount: command.Amount,
	})
	if err != nil {
		t.Fatal(err)
	}

	first, firstReplay := invokeControlledTransfer(t, session.db, command, fingerprint[:])
	second, secondReplay := invokeControlledTransfer(t, session.db, command, fingerprint[:])
	if firstReplay || !secondReplay || first.TransferID == "" || first.TransferID != second.TransferID || first.Status != "posted" || first.AmountMinor != 100 {
		t.Fatalf("unexpected controlled transfer/replay first=%#v first_replay=%t second=%#v second_replay=%t", first, firstReplay, second, secondReplay)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers WHERE id=$1 AND status='posted'`, first.TransferID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM journal_transactions WHERE transfer_id=$1`, first.TransferID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM ledger_postings posting JOIN journal_transactions journal ON journal.id=posting.journal_transaction_id WHERE journal.transfer_id=$1`, first.TransferID) != 2 {
		t.Fatal("controlled transfer did not commit exactly one canonical journal")
	}
}

func TestControlledTransferFunctionRejectsSpoofingAndRollsBackPartialState(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	api := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")
	support := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_support_readonly")

	t.Run("unauthorized actor", func(t *testing.T) {
		command := transferCommand(t, "controlled-spoof-actor-0001", "1.00")
		command.ActorSubjectID = "spoofed-finance-actor"
		fingerprint := transferFingerprint(t, command)
		_, err := api.db.ExecContext(context.Background(), `SELECT * FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount.Minor(), command.Amount.Currency().Code,
			command.IdempotencyKey, fingerprint, command.CorrelationID)
		if sqlState(err) != "42501" {
			t.Fatalf("actor spoof SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
		}
		if countRows(t, database, `SELECT count(*) FROM idempotency_requests WHERE idempotency_key=$1`, command.IdempotencyKey) != 0 {
			t.Fatal("failed actor spoof left a partial idempotency reservation")
		}
	})

	t.Run("cross tenant account", func(t *testing.T) {
		const otherTenant = "00000000-0000-4000-8000-000000008001"
		const otherAccount = "00000000-0000-4000-8000-000000008002"
		if _, err := database.Exec(`INSERT INTO tenants(id,external_reference) VALUES($1,'controlled-other-tenant')`, otherTenant); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference) VALUES($1,$2,'USD','active','Other tenant','operating','controlled-other')`, otherAccount, otherTenant); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version) VALUES($1,1000,1000,0)`, otherAccount); err != nil {
			t.Fatal(err)
		}
		command := transferCommand(t, "controlled-cross-tenant-0001", "1.00")
		command.DebitAccountID = otherAccount
		fingerprint := transferFingerprint(t, command)
		_, err := api.db.ExecContext(context.Background(), `SELECT * FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount.Minor(), command.Amount.Currency().Code,
			command.IdempotencyKey, fingerprint, command.CorrelationID)
		if sqlState(err) != "42501" {
			t.Fatalf("cross-tenant command SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
		}
		if countRows(t, database, `SELECT count(*) FROM transfers WHERE actor_subject_id=$1 AND debit_account_id=$2`, command.ActorSubjectID, otherAccount) != 0 {
			t.Fatal("cross-tenant command committed a transfer")
		}
	})

	t.Run("search path poisoning", func(t *testing.T) {
		command := transferCommand(t, "controlled-search-path-0001", "0.01")
		fingerprint := transferFingerprint(t, command)
		tx, err := api.db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err = tx.Exec(`CREATE TEMP TABLE transfers(id uuid); SET LOCAL search_path=pg_temp,public`); err != nil {
			t.Fatal(err)
		}
		var response []byte
		if err = tx.QueryRow(`SELECT response_body FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount.Minor(), command.Amount.Currency().Code,
			command.IdempotencyKey, fingerprint, command.CorrelationID).Scan(&response); err != nil {
			t.Fatalf("fixed search path execution: %v", err)
		}
		if err = tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("support role denied", func(t *testing.T) {
		command := transferCommand(t, "controlled-support-denied-0001", "0.01")
		fingerprint := transferFingerprint(t, command)
		_, err := support.db.ExecContext(context.Background(), `SELECT * FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount.Minor(), command.Amount.Currency().Code,
			command.IdempotencyKey, fingerprint, command.CorrelationID)
		if sqlState(err) != "42501" {
			t.Fatalf("support execution SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
		}
	})
}

func invokeControlledTransfer(t *testing.T, database queryRower, command apptransfers.Command, fingerprint []byte) (apptransfers.Result, bool) {
	t.Helper()
	var response []byte
	var replayed bool
	err := database.QueryRowContext(context.Background(), `SELECT response_body,replayed FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount.Minor(), command.Amount.Currency().Code,
		command.IdempotencyKey, fingerprint, command.CorrelationID).Scan(&response, &replayed)
	if err != nil {
		t.Fatal(err)
	}
	var result apptransfers.Result
	if err := json.Unmarshal(response, &result); err != nil {
		t.Fatalf("decode controlled transfer response: %v", err)
	}
	return result, replayed
}

func transferFingerprint(t *testing.T, command apptransfers.Command) []byte {
	t.Helper()
	fingerprint, err := apptransfers.Fingerprint(apptransfers.IdempotencyRequest{
		TenantID: command.TenantID, ActorSubjectID: command.ActorSubjectID, Operation: "transfers.create.v1", Key: command.IdempotencyKey,
		DebitAccountID: command.DebitAccountID, CreditAccountID: command.CreditAccountID, Amount: command.Amount,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint[:]
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("LEDGERSYNC_TEST_DATABASE_URL is required")
	}
	return databaseURL
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
