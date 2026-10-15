package integration_test

import (
	"context"
	"errors"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"testing"
	"time"
)

func TestFundingRequestAndPostingApprovalWindowsFailClosed(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	const approver = "synthetic-expiry-approver"
	if _, err := admin.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance')`, testTenantID, testActorID, approver); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',true,1,100000,200000,500000)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	clock := func() time.Time { return now }
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewFundingRepository(api.db, clock)
	service, _ := appfunding.NewService(repository, appfunding.PolicyProductionDualControl, clock)
	amount, _ := money.New("USD", 100)
	ctx := context.Background()
	request := appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount, ExternalReference: "synthetic-expired-request", EvidenceReference: "evidence://synthetic/expiry", IdempotencyKey: "funding-request-expiry-001", CorrelationID: "00000000-0000-4000-8000-000000000996"}
	first, err := service.Request(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	decision := appfunding.DecisionCommand{TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: first.Event.FundingEventID, Reason: "Synthetic expiry review", CorrelationID: request.CorrelationID}
	if _, err := service.Approve(ctx, decision); !errors.Is(err, appfunding.ErrForbidden) {
		t.Fatalf("expired request approval accepted: %v", err)
	}
	if _, err := service.Reject(ctx, decision); err != nil {
		t.Fatalf("expired request cannot be rejected: %v", err)
	}
	request.ExternalReference = "synthetic-expired-posting"
	request.IdempotencyKey = "funding-posting-expiry-001"
	second, err := service.Request(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	decision.FundingEventID = second.Event.FundingEventID
	if _, err := service.Approve(ctx, decision); err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	if _, err := service.Post(ctx, appfunding.ActionCommand{TenantID: testTenantID, ActorSubjectID: approver, FundingEventID: second.Event.FundingEventID, IdempotencyKey: "expired-funding-post-001", CorrelationID: request.CorrelationID}); !errors.Is(err, appfunding.ErrForbidden) {
		t.Fatalf("expired posting approval accepted: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM ledger_postings`); got != 0 {
		t.Fatal("expired funding moved money")
	}
}
