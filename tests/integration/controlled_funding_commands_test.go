package integration_test

import (
	"context"
	"testing"
	"time"

	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

const controlledFundingFunction = `public.controlled_post_funding_v1(uuid,text,uuid,text,uuid,timestamptz)`

func TestControlledFundingFunctionUsesFixedDefinerBoundary(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)

	var owner, searchPath string
	var securityDefiner, apiCanExecute, supportCanExecute, ownerCanUpdateEvent, ownerCanWriteLedger, publicCanExecute bool
	err := database.QueryRowContext(context.Background(), `
SELECT owner.rolname,
       procedure.prosecdef,
       COALESCE((SELECT setting FROM unnest(procedure.proconfig) setting WHERE setting LIKE 'search_path=%'),''),
       has_function_privilege('ledgersync_api',$1,'EXECUTE'),
       has_function_privilege('ledgersync_support_readonly',$1,'EXECUTE'),
       has_table_privilege('ledgersync_migration_owner','public.funding_events','UPDATE'),
       has_table_privilege('ledgersync_migration_owner','public.ledger_postings','INSERT'),
       EXISTS(
         SELECT 1 FROM aclexplode(procedure.proacl) acl
         WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'
       )
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
JOIN pg_roles owner ON owner.oid=procedure.proowner
WHERE namespace.nspname='public' AND procedure.proname='controlled_post_funding_v1'`, controlledFundingFunction).
		Scan(&owner, &securityDefiner, &searchPath, &apiCanExecute, &supportCanExecute, &ownerCanUpdateEvent, &ownerCanWriteLedger, &publicCanExecute)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ledgersync_migration_owner" || !securityDefiner || searchPath != "search_path=pg_catalog, public" || !apiCanExecute || supportCanExecute || !ownerCanUpdateEvent || !ownerCanWriteLedger || publicCanExecute {
		t.Fatalf("unsafe controlled funding metadata owner=%q definer=%t search_path=%q api=%t support=%t owner_event_update=%t owner_ledger_write=%t public=%t", owner, securityDefiner, searchPath, apiCanExecute, supportCanExecute, ownerCanUpdateEvent, ownerCanWriteLedger, publicCanExecute)
	}
}

func TestControlledFundingFunctionExecutesAsAPIAndRejectsSpoofing(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	ctx := context.Background()
	const approver = "controlled-funding-approver"
	if _, err := database.ExecContext(ctx, `
INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES
  ($1,$2,'finance'),($1,$3,'finance')`, testTenantID, testActorID, approver); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO tenant_funding_policies(
  tenant_id,currency,mode,finance_activated,policy_version,
  per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor
) VALUES($1,'USD','production_dual_control',true,1,100000,200000,500000)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository, err := db.NewFundingRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service, err := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New("USD", 5_000)
	created, err := service.Request(ctx, appfunding.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount,
		ExternalReference: "controlled-funding-evidence-001", EvidenceReference: "evidence://controlled/funding/001",
		IdempotencyKey: "controlled-funding-request-001", CorrelationID: "00000000-0000-4000-8000-000000008501",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Approve(ctx, appfunding.DecisionCommand{
		TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: created.Event.FundingEventID,
		Reason: "controlled evidence verified", CorrelationID: "00000000-0000-4000-8000-000000008502",
	}); err != nil {
		t.Fatal(err)
	}

	api := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")
	support := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_support_readonly")
	const postKey = "controlled-funding-post-0001"
	const correlationID = "00000000-0000-4000-8000-000000008503"
	var replayed bool
	if err = api.db.QueryRowContext(ctx, `SELECT replayed FROM public.controlled_post_funding_v1($1,$2,$3,$4,$5,$6)`,
		testTenantID, approver, created.Event.FundingEventID, postKey, correlationID, now).Scan(&replayed); err != nil || replayed {
		t.Fatalf("API controlled funding post replayed=%t error=%v", replayed, err)
	}
	if countRows(t, database, `SELECT count(*) FROM journal_transactions WHERE funding_event_id=$1`, created.Event.FundingEventID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM ledger_postings posting JOIN journal_transactions journal ON journal.id=posting.journal_transaction_id WHERE journal.funding_event_id=$1`, created.Event.FundingEventID) != 2 {
		t.Fatal("controlled funding post did not commit one canonical journal")
	}

	if _, err = api.db.ExecContext(ctx, `SELECT * FROM public.controlled_post_funding_v1($1,$2,$3,$4,$5,$6)`,
		testTenantID, "spoofed-finance-actor", created.Event.FundingEventID, postKey, correlationID, now); sqlState(err) != "42501" {
		t.Fatalf("spoofed actor SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}
	if _, err = support.db.ExecContext(ctx, `SELECT * FROM public.controlled_post_funding_v1($1,$2,$3,$4,$5,$6)`,
		testTenantID, approver, created.Event.FundingEventID, postKey, correlationID, now); sqlState(err) != "42501" {
		t.Fatalf("support execution SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`CREATE TEMP TABLE funding_events(id uuid); SET LOCAL search_path=pg_temp,public`); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(`SELECT replayed FROM public.controlled_post_funding_v1($1,$2,$3,$4,$5,$6)`,
		testTenantID, approver, created.Event.FundingEventID, postKey, correlationID, now).Scan(&replayed); err != nil || !replayed {
		t.Fatalf("fixed search path replayed=%t error=%v", replayed, err)
	}
}
