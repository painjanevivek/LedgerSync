package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestFundingClearingCreationWorksWithoutDirectAccountInsert(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	if _, err := admin.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance')`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',true,1,100000,200000,500000)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	// Exercise the future grant boundary before committing the role cutover.
	if _, err := admin.Exec(`REVOKE INSERT ON accounts,account_opening_balances,account_balance_projections,account_owners,account_credit_permissions FROM ledgersync_api`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provisionLiveMigrationRoles(t, admin) })
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewFundingRepository(api.db, time.Now)
	service, _ := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, time.Now)
	amount, _ := money.New("USD", 100)
	command := appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount, ExternalReference: "synthetic-controlled-funding", EvidenceReference: "evidence://synthetic/controlled", IdempotencyKey: "controlled-clearing-request-001", CorrelationID: "00000000-0000-4000-8000-000000000991"}
	first, err := service.Request(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Request(ctx, command)
	if err != nil || !replay.Replayed || replay.Event.FundingEventID != first.Event.FundingEventID {
		t.Fatalf("funding request replay: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM accounts a JOIN account_balance_projections b ON a.id=b.account_id JOIN account_opening_balances o ON a.id=o.account_id WHERE a.tenant_id=$1 AND a.account_kind='funding_clearing' AND b.available_minor=0 AND b.ledger_minor=0 AND b.balance_version=0 AND b.allow_negative AND o.opening_ledger_minor=0`, testTenantID); got != 1 {
		t.Fatalf("exact zero clearing state count=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM ledger_postings`); got != 0 {
		t.Fatal("request setup moved money")
	}
	var ignored string
	err = tenantTestRow(ctx, api.db, testTenantID, testActorID, `SELECT controlled_ensure_funding_account_v1($1,$2,'USD',$3,now())`, testTenantID, "spoofed-actor", command.CorrelationID).Scan(&ignored)
	if sqlState(err) != "42501" {
		t.Fatalf("spoofed funding account actor accepted: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM audit_events WHERE event_type='account.funding_clearing_created'`); got != 1 {
		t.Fatalf("clearing creation audit count=%d", got)
	}
}

func TestConcurrentFundingAccountCreationHasOneZeroOpening(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	const peer = "synthetic-funding-peer"
	if _, err := admin.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance')`, testTenantID, testActorID, peer); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',true,1,100000,200000,500000)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewFundingRepository(api.db, time.Now)
	service, _ := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, time.Now)
	amount, _ := money.New("USD", 100)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for i, actor := range []string{testActorID, peer} {
		group.Add(1)
		go func(i int, actor string) {
			defer group.Done()
			<-start
			destination := testDestinationID
			if i == 1 {
				destination = testSourceID
			}
			_, err := service.Request(ctx, appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: actor, DestinationAccountID: destination, Amount: amount, ExternalReference: "synthetic-concurrent-" + actor, EvidenceReference: "evidence://synthetic/concurrent", IdempotencyKey: "controlled-concurrent-" + actor, CorrelationID: "00000000-0000-4000-8000-000000000992"})
			results <- err
		}(i, actor)
	}
	close(start)
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := countRows(t, admin, `SELECT count(*) FROM accounts WHERE tenant_id=$1 AND account_kind='funding_clearing'`, testTenantID); got != 1 {
		t.Fatalf("clearing account count=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM funding_events WHERE tenant_id=$1 AND status='requested'`, testTenantID); got != 2 {
		t.Fatalf("funding request count=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM audit_events WHERE event_type='account.funding_clearing_created'`); got != 1 {
		t.Fatalf("creation audit count=%d", got)
	}
}
