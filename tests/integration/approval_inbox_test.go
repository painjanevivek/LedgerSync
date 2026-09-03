package integration_test

import (
	"context"
	"testing"
	"time"

	appapprovals "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/approvals"
	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestApprovalInboxCombinesTypedEvidenceInStableOldestFirstPages(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	const approver = "approval-inbox-finance"
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
	fundingTime := time.Date(2026, 8, 18, 9, 20, 0, 0, time.UTC)
	fundingRepository, err := db.NewFundingRepository(database, func() time.Time { return fundingTime })
	if err != nil {
		t.Fatal(err)
	}
	fundingService, err := appfunding.NewService(fundingRepository, appfunding.PolicyProductionDualControl, func() time.Time { return fundingTime })
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New("USD", 1_250)
	funding, err := fundingService.Request(ctx, appfunding.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount,
		ExternalReference: "approval-inbox-wire-001", EvidenceReference: "evidence://approval-inbox/wire-001",
		IdempotencyKey: "approval-inbox-funding-0001", CorrelationID: "00000000-0000-4000-8000-000000000951",
	})
	if err != nil {
		t.Fatal(err)
	}

	original, err := transferService.Submit(ctx, transferCommand(t, "approval-inbox-transfer-0001", "10.00"))
	if err != nil {
		t.Fatal(err)
	}
	correctionTime := time.Date(2026, 8, 18, 9, 40, 0, 0, time.UTC)
	correctionRepository, err := db.NewTransferCorrectionRepository(database, func() time.Time { return correctionTime })
	if err != nil {
		t.Fatal(err)
	}
	correctionService, err := appcorrections.NewService(correctionRepository, func() time.Time { return correctionTime })
	if err != nil {
		t.Fatal(err)
	}
	correction, err := correctionService.Request(ctx, appcorrections.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID,
		ReasonCode: "operational_error", OperatorNote: "Independent review evidence for the approval inbox.",
		IdempotencyKey: "approval-inbox-correction-0001", CorrelationID: "00000000-0000-4000-8000-000000000952",
		StepUpAuthenticatedAt: correctionTime.Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	inboxRepository, err := db.NewApprovalRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := appapprovals.NewService(inboxRepository, func() time.Time { return time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	query := appapprovals.Query{Limit: 1, CanApproveFunding: true, CanApproveCorrections: true, StepUpAuthenticatedAt: time.Date(2026, 8, 18, 9, 55, 0, 0, time.UTC)}
	first, err := inbox.List(ctx, testTenantID, approver, query)
	if err != nil || len(first.Items) != 1 || first.Items[0].Domain != appapprovals.DomainFunding || first.Items[0].RecordID != funding.Event.FundingEventID || first.Items[0].SafeNextAction != "review_decision" || first.NextCursor == "" {
		t.Fatalf("first approval page=%#v err=%v", first, err)
	}
	query.Cursor = first.NextCursor
	second, err := inbox.List(ctx, testTenantID, approver, query)
	if err != nil || len(second.Items) != 1 || second.Items[0].Domain != appapprovals.DomainCorrection || second.Items[0].RecordID != correction.Event.CorrectionID || second.Items[0].StepUpStatus != appapprovals.StepUpSatisfied || second.NextCursor != "" {
		t.Fatalf("second approval page=%#v err=%v", second, err)
	}
	if first.PageCount != 1 || second.PageCount != 1 || first.Items[0].AgeSeconds != "2400" || second.Items[0].AgeSeconds != "1200" {
		t.Fatalf("approval page counts or server ages are not exact: first=%#v second=%#v", first, second)
	}
}
