package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestAccountLifecycleWithoutDirectFinancialUpdate(t *testing.T) {
	_, admin := requireAccountCommandService(t)
	requireWorkloadRoles(t, admin)
	if _, err := admin.Exec(`REVOKE UPDATE ON accounts,account_balance_projections FROM ledgersync_api`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provisionLiveMigrationRoles(t, admin) })
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	clock := func() time.Time { return accountCommandTime }
	repository, _ := db.NewAccountCommandRepository(api.db, clock)
	service, _ := accounts.NewCommandService(repository, clock)
	ctx := context.Background()
	created, err := service.Create(ctx, createAccountCommand("restricted-lifecycle-create", "restricted-lifecycle"))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateMetadata(ctx, accounts.UpdateAccountMetadataCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", IdempotencyKey: "restricted-lifecycle-metadata", AccountID: created.Result.AccountID, ExpectedVersion: 1, DisplayName: "Reserve", Reference: "restricted-reserve", Category: "reserve"})
	if err != nil || updated.Result.Version != "2" {
		t.Fatalf("metadata: %v, %+v", err, updated)
	}
	command := accounts.ChangeAccountStatusCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", AccountID: created.Result.AccountID, Reason: "Synthetic lifecycle verification"}
	for i, status := range []accountdomain.Status{accountdomain.StatusFrozen, accountdomain.StatusActive, accountdomain.StatusClosed} {
		command.IdempotencyKey = "restricted-lifecycle-" + string(status)
		command.ExpectedVersion = int64(i + 2)
		command.TargetStatus = status
		result, err := service.ChangeStatus(ctx, command)
		if err != nil || result.Result.Status != string(status) {
			t.Fatalf("%s: %v", status, err)
		}
		replay, err := service.ChangeStatus(ctx, command)
		if err != nil || !replay.Replayed || replay.Result != result.Result {
			t.Fatalf("%s replay: %v", status, err)
		}
	}
	command.IdempotencyKey = "restricted-lifecycle-terminal"
	command.ExpectedVersion = 5
	command.TargetStatus = accountdomain.StatusActive
	if _, err := service.ChangeStatus(ctx, command); !errors.Is(err, accounts.ErrTerminalStatus) {
		t.Fatalf("terminal guard: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM audit_events WHERE target_id=$1 AND event_type IN ('account.metadata_updated','account.status_changed')`, created.Result.AccountID); got != 4 {
		t.Fatalf("mutation audit count=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE account_id=$1`, created.Result.AccountID); got != 5 {
		t.Fatalf("event count=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM ledger_postings`); got != 0 {
		t.Fatal("lifecycle moved money")
	}
}

func TestControlledAccountCloseAndAuditCannotBeBypassed(t *testing.T) {
	_, admin := requireAccountCommandService(t)
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	ctx := context.Background()
	var name, reference, category string
	if err := admin.QueryRow(`SELECT display_name,external_reference,category FROM accounts WHERE id=$1`, testSourceID).Scan(&name, &reference, &category); err != nil {
		t.Fatal(err)
	}
	_, err := tenantTestExec(ctx, api.db, testTenantID, testActorID, `SELECT controlled_update_account_v1($1,$2,$3,1,$4,$5,$6,'closed',$7,$8,'Synthetic direct close')`, testTenantID, testActorID, testSourceID, name, reference, category, "00000000-0000-0000-0000-000000000099", accountCommandTime)
	if sqlState(err) != "23514" {
		t.Fatalf("direct nonzero closure accepted: %v", err)
	}
	clock := func() time.Time { return accountCommandTime }
	repository, _ := db.NewAccountCommandRepository(api.db, clock)
	service, _ := accounts.NewCommandService(repository, clock)
	created, err := service.Create(ctx, createAccountCommand("controlled-audit-create-001", "controlled-audit"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`CREATE FUNCTION fail_lifecycle_audit_for_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type='account.status_changed' THEN RAISE EXCEPTION 'injected audit failure' USING ERRCODE='58000'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_lifecycle_audit_for_test BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_lifecycle_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`DROP TRIGGER IF EXISTS fail_lifecycle_audit_for_test ON audit_events; DROP FUNCTION IF EXISTS fail_lifecycle_audit_for_test()`)
	})
	command := accounts.ChangeAccountStatusCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", IdempotencyKey: "controlled-audit-freeze-001", AccountID: created.Result.AccountID, ExpectedVersion: 1, TargetStatus: accountdomain.StatusFrozen, Reason: "Synthetic atomic audit check"}
	if _, err := service.ChangeStatus(ctx, command); !errors.Is(err, accounts.ErrCommandUnavailable) {
		t.Fatalf("audit failure: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM accounts WHERE id=$1 AND status='active' AND version=1`, created.Result.AccountID); got != 1 {
		t.Fatal("audit failure changed account")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE account_id=$1 AND event_type='account.status.changed.v1'`, created.Result.AccountID); got != 0 {
		t.Fatal("audit failure published status change")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM idempotency_requests WHERE idempotency_key=$1`, command.IdempotencyKey); got != 0 {
		t.Fatal("audit failure retained a partial outcome")
	}
	if _, err := admin.Exec(`DROP TRIGGER fail_lifecycle_audit_for_test ON audit_events; DROP FUNCTION fail_lifecycle_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	if result, err := service.ChangeStatus(ctx, command); err != nil || result.Replayed || result.Result.Status != "frozen" {
		t.Fatalf("retry after audit recovery: %v", err)
	}
}
