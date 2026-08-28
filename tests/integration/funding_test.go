package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	appaccounts "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestFundingRequestApprovalPostingReplayAndReconciliationAreAtomic(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	const approver = "integration-finance-approver"
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
	clock := func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	repository, err := db.NewFundingRepository(database, clock)
	if err != nil {
		t.Fatal(err)
	}
	service, err := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, clock)
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New("USD", 5_000)
	request := appfunding.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount,
		ExternalReference: "customer-wire-2026-001", EvidenceReference: "evidence://customer/wire/2026-001",
		IdempotencyKey: "funding-request-key-000001", CorrelationID: "00000000-0000-4000-8000-000000000501",
	}
	created, err := service.Request(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Request(ctx, request)
	if err != nil || !replayed.Replayed || replayed.Event.FundingEventID != created.Event.FundingEventID {
		t.Fatalf("request replay=%#v err=%v", replayed, err)
	}
	conflicting := request
	conflicting.Amount, _ = money.New("USD", 5_001)
	if _, err = service.Request(ctx, conflicting); !errors.Is(err, appfunding.ErrConflict) {
		t.Fatalf("conflicting key error=%v", err)
	}
	if _, err = service.Approve(ctx, appfunding.DecisionCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, FundingEventID: created.Event.FundingEventID,
		Reason: "self approval", CorrelationID: "00000000-0000-4000-8000-000000000502",
	}); !errors.Is(err, appfunding.ErrForbidden) {
		t.Fatalf("production self approval error=%v", err)
	}
	approved, err := service.Approve(ctx, appfunding.DecisionCommand{
		TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: created.Event.FundingEventID,
		Reason: "customer evidence verified", CorrelationID: "00000000-0000-4000-8000-000000000503",
	})
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approval=%#v err=%v", approved, err)
	}
	posted, err := service.Post(ctx, appfunding.ActionCommand{
		TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: created.Event.FundingEventID,
		CorrelationID: "00000000-0000-4000-8000-000000000504",
	})
	if err != nil || posted.Event.Status != "posted" || posted.Event.BalanceVersion != "1" {
		t.Fatalf("posted=%#v err=%v", posted, err)
	}
	postReplay, err := service.Post(ctx, appfunding.ActionCommand{
		TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: created.Event.FundingEventID,
		CorrelationID: "00000000-0000-4000-8000-000000000504",
	})
	if err != nil || !postReplay.Replayed || postReplay.Event.JournalTransactionID != posted.Event.JournalTransactionID {
		t.Fatalf("post replay=%#v err=%v", postReplay, err)
	}
	if countRows(t, database, `SELECT count(*) FROM funding_events WHERE tenant_id=$1`, testTenantID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM journal_transactions WHERE funding_event_id=$1`, created.Event.FundingEventID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM ledger_postings WHERE journal_transaction_id=$1`, posted.Event.JournalTransactionID) != 2 ||
		countRows(t, database, `SELECT count(*) FROM approval_records WHERE target_id=$1 AND status='approved'`, created.Event.FundingEventID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM audit_events WHERE target_id=$1 AND event_type='funding.posted'`, created.Event.FundingEventID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM outbox_events WHERE funding_event_id=$1`, created.Event.FundingEventID) != 1 {
		t.Fatal("funding transaction evidence was incomplete or duplicated")
	}
	var destinationBalance int64
	if err = database.QueryRowContext(ctx, `SELECT ledger_minor FROM account_balance_projections WHERE account_id=$1`, testDestinationID).Scan(&destinationBalance); err != nil || destinationBalance != 7_000 {
		t.Fatalf("destination balance=%d err=%v", destinationBalance, err)
	}
	reconciled, err := service.Reconcile(ctx, testTenantID, approver, created.Event.FundingEventID)
	if err != nil || reconciled.Status != "matched" || reconciled.ExpectedMinor != "5000" || reconciled.PostedDebitMinor != "5000" || reconciled.PostedCreditMinor != "5000" {
		t.Fatalf("reconciliation=%#v err=%v", reconciled, err)
	}
	var systemBalance int64
	var systemAllowsNegative bool
	if err = database.QueryRowContext(ctx, `SELECT ledger_minor,allow_negative FROM account_balance_projections WHERE account_id=$1`, posted.Event.SystemAccountID).Scan(&systemBalance, &systemAllowsNegative); err != nil || systemBalance != -5_000 || !systemAllowsNegative {
		t.Fatalf("funding clearing balance=%d allows_negative=%t err=%v", systemBalance, systemAllowsNegative, err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO account_owners(tenant_id,account_id,subject_id,permission,created_at) VALUES($1,$2,$3,'read',$4)`, testTenantID, posted.Event.SystemAccountID, testActorID, clock()); err != nil {
		t.Fatal(err)
	}
	accountRepository, err := db.NewAccountRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	accountPage, err := accountRepository.ListOwnedPage(ctx, testTenantID, testActorID, appaccounts.Query{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accountPage.Accounts {
		if account.AccountID == posted.Event.SystemAccountID {
			t.Fatal("funding clearing account leaked into customer account directory")
		}
	}
	duplicateEvidence := request
	duplicateEvidence.ActorSubjectID = approver
	duplicateEvidence.IdempotencyKey = "funding-request-key-duplicate-001"
	duplicateEvidence.CorrelationID = "00000000-0000-4000-8000-000000000508"
	if _, err = service.Request(ctx, duplicateEvidence); !errors.Is(err, appfunding.ErrConflict) {
		t.Fatalf("duplicate external evidence error=%v", err)
	}

	compensationCommand := appfunding.CompensationCommand{
		TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: created.Event.FundingEventID,
		ReasonCode: "external_evidence_reversed", OperatorNote: "Verified reversal against external evidence case 2026-001.",
		IdempotencyKey: "funding-compensation-key-000001", CorrelationID: "00000000-0000-4000-8000-000000000505",
	}
	compensation, err := service.Compensate(ctx, compensationCommand)
	if err != nil || compensation.Event.Status != "requested" || compensation.Event.CompensationOfEventID != created.Event.FundingEventID {
		t.Fatalf("compensation=%#v err=%v", compensation, err)
	}
	compensationReplay, err := service.Compensate(ctx, compensationCommand)
	if err != nil || !compensationReplay.Replayed || compensationReplay.Event.FundingEventID != compensation.Event.FundingEventID {
		t.Fatalf("compensation replay=%#v err=%v", compensationReplay, err)
	}
	firstPage, err := service.List(ctx, testTenantID, approver, appfunding.Query{Limit: 1})
	if err != nil || len(firstPage.Events) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first funding page=%#v err=%v", firstPage, err)
	}
	secondPage, err := service.List(ctx, testTenantID, approver, appfunding.Query{Limit: 1, Cursor: firstPage.NextCursor})
	if err != nil || len(secondPage.Events) != 1 || secondPage.Events[0].FundingEventID == firstPage.Events[0].FundingEventID {
		t.Fatalf("second funding page=%#v err=%v", secondPage, err)
	}
	if _, err = service.List(ctx, testTenantID, approver, appfunding.Query{Limit: 1, Cursor: "not-a-valid-cursor"}); !errors.Is(err, appfunding.ErrInvalidCommand) {
		t.Fatalf("invalid funding cursor error=%v", err)
	}
	if _, err = service.Approve(ctx, appfunding.DecisionCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, FundingEventID: compensation.Event.FundingEventID,
		Reason: "reversal evidence independently verified", CorrelationID: "00000000-0000-4000-8000-000000000506",
	}); err != nil {
		t.Fatal(err)
	}
	compensated, err := service.Post(ctx, appfunding.ActionCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, FundingEventID: compensation.Event.FundingEventID,
		CorrelationID: "00000000-0000-4000-8000-000000000507",
	})
	if err != nil || compensated.Event.Status != "posted" || compensated.Event.BalanceVersion != "2" {
		t.Fatalf("compensated=%#v err=%v", compensated, err)
	}
	original, err := service.Get(ctx, testTenantID, testActorID, created.Event.FundingEventID)
	if err != nil || original.Status != "compensated" || original.CompensationEventID != compensation.Event.FundingEventID {
		t.Fatalf("original after compensation=%#v err=%v", original, err)
	}
	if err = database.QueryRowContext(ctx, `SELECT ledger_minor FROM account_balance_projections WHERE account_id=$1`, testDestinationID).Scan(&destinationBalance); err != nil || destinationBalance != 2_000 {
		t.Fatalf("destination after compensation=%d err=%v", destinationBalance, err)
	}
	if err = database.QueryRowContext(ctx, `SELECT ledger_minor FROM account_balance_projections WHERE account_id=$1`, posted.Event.SystemAccountID).Scan(&systemBalance); err != nil || systemBalance != 0 {
		t.Fatalf("clearing after compensation=%d err=%v", systemBalance, err)
	}
	if countRows(t, database, `SELECT count(*) FROM approval_records WHERE target_id=$1 AND command_type='funding_compensation'`, compensation.Event.FundingEventID) != 2 ||
		countRows(t, database, `SELECT count(*) FROM funding_velocity_events WHERE tenant_id=$1`, testTenantID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM journal_transactions WHERE funding_event_id=$1`, compensation.Event.FundingEventID) != 1 {
		t.Fatal("compensation evidence or velocity accounting was incorrect")
	}
	secondCompensation := compensationCommand
	secondCompensation.IdempotencyKey = "funding-compensation-key-000002"
	if _, err = service.Compensate(ctx, secondCompensation); !errors.Is(err, appfunding.ErrConflict) {
		t.Fatalf("second compensation error=%v", err)
	}
	if _, err = database.ExecContext(ctx, `UPDATE funding_events SET amount_minor=amount_minor+1 WHERE id=$1`, created.Event.FundingEventID); err == nil {
		t.Fatal("final funding evidence was mutable")
	}
	if _, err = database.ExecContext(ctx, `DELETE FROM approval_records WHERE target_id=$1`, compensation.Event.FundingEventID); err == nil {
		t.Fatal("approval evidence was mutable")
	}
}

func TestFundingLimitsAndFinanceActivationFailClosed(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance')`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',false,1,100,150,200)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewFundingRepository(database, nil)
	service, _ := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, nil)
	amount, _ := money.New("USD", 101)
	_, err := service.Request(ctx, appfunding.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount,
		ExternalReference: "external-limit", EvidenceReference: "evidence://limit", IdempotencyKey: "funding-limit-key-000001",
		CorrelationID: "00000000-0000-4000-8000-000000000510",
	})
	if !errors.Is(err, appfunding.ErrForbidden) && !errors.Is(err, appfunding.ErrLimitExceeded) {
		t.Fatalf("inactive/over-limit policy error=%v", err)
	}
	if countRows(t, database, `SELECT count(*) FROM funding_events WHERE tenant_id=$1`, testTenantID) != 0 {
		t.Fatal("denied funding left financial state")
	}
	if _, err = database.ExecContext(ctx, `UPDATE account_balance_projections SET allow_negative=true WHERE account_id=$1`, testDestinationID); err == nil {
		t.Fatal("customer account was allowed to opt into negative balances")
	}
}
