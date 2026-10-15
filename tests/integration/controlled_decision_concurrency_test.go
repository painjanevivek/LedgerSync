package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func assertOneOpposedDecisionWins(t *testing.T, approve, reject func() error, conflict error) {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, action := range []func() error{approve, reject} {
		group.Add(1)
		go func(action func() error) { defer group.Done(); <-start; results <- action() }(action)
	}
	close(start)
	group.Wait()
	close(results)
	success, denied := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, conflict) {
			denied++
		} else {
			t.Fatalf("opposed decision dependency failure: %v", err)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatalf("opposed decisions success=%d conflict=%d", success, denied)
	}
}

func TestControlledFundingOpposedDecisionsCommitOneOutcome(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	const approver = "synthetic-decision-approver"
	const rejector = "synthetic-decision-rejector"
	if _, err := admin.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance'),($1,$4,'finance')`, testTenantID, testActorID, approver, rejector); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',true,1,100000,200000,500000)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewFundingRepository(api.db, time.Now)
	service, _ := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	amount, _ := money.New("USD", 100)
	created, err := service.Request(ctx, appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount, ExternalReference: "synthetic-opposed-funding", EvidenceReference: "evidence://synthetic/opposed", IdempotencyKey: "funding-opposed-request-001", CorrelationID: "00000000-0000-4000-8000-000000000994"})
	if err != nil {
		t.Fatal(err)
	}
	command := appfunding.DecisionCommand{TenantID: testTenantID, FundingEventID: created.Event.FundingEventID, Reason: "Synthetic opposed decision", CorrelationID: "00000000-0000-4000-8000-000000000994"}
	assertOneOpposedDecisionWins(t, func() error { c := command; c.ActorSubjectID = approver; _, err := service.Approve(ctx, c); return err }, func() error { c := command; c.ActorSubjectID = rejector; _, err := service.Reject(ctx, c); return err }, appfunding.ErrConflict)
	if got := countRows(t, admin, `SELECT count(*) FROM approval_records WHERE target_id=$1 AND status IN ('approved','rejected')`, created.Event.FundingEventID); got != 1 {
		t.Fatalf("funding decision evidence=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM ledger_postings`); got != 0 {
		t.Fatal("approval moved money")
	}
}

func TestControlledCorrectionOpposedDecisionsCommitOneOutcome(t *testing.T) {
	transfers, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	const approver = "synthetic-correction-approver"
	const rejector = "synthetic-correction-rejector"
	if _, err := admin.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance')`, testTenantID, approver, rejector); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`UPDATE tenant_transfer_policies SET control_mode='production_dual_control',requires_step_up=false WHERE tenant_id=$1`, testTenantID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	original, err := transfers.Submit(ctx, transferCommand(t, "correction-opposed-original", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewTransferCorrectionRepository(api.db, time.Now)
	service, _ := appcorrections.NewService(repository, time.Now)
	created, err := service.Request(ctx, appcorrections.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID, ReasonCode: "operational_error", OperatorNote: "Synthetic opposed correction", IdempotencyKey: "correction-opposed-request", CorrelationID: "00000000-0000-4000-8000-000000000995"})
	if err != nil {
		t.Fatal(err)
	}
	command := appcorrections.DecisionCommand{TenantID: testTenantID, CorrectionID: created.Event.CorrectionID, Reason: "Synthetic opposed decision", CorrelationID: "00000000-0000-4000-8000-000000000995"}
	assertOneOpposedDecisionWins(t, func() error { c := command; c.ActorSubjectID = approver; _, err := service.Approve(ctx, c); return err }, func() error { c := command; c.ActorSubjectID = rejector; _, err := service.Reject(ctx, c); return err }, appcorrections.ErrConflict)
	if got := countRows(t, admin, `SELECT count(*) FROM approval_records WHERE target_id=$1 AND status IN ('approved','rejected')`, created.Event.CorrectionID); got != 1 {
		t.Fatalf("correction decision evidence=%d", got)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM transfers`); got != 1 {
		t.Fatal("correction approval created a transfer")
	}
}
