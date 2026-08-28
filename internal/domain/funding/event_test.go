package funding_test

import (
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

func requestedEvent(t *testing.T) funding.Event {
	t.Helper()
	amount, err := money.New("USD", 25_000)
	if err != nil {
		t.Fatal(err)
	}
	event, err := funding.Request(funding.RequestInput{
		ID: "00000000-0000-0000-0000-000000000501", TenantID: "00000000-0000-0000-0000-000000000502",
		RequesterSubjectID: "finance-requester", DestinationAccountID: "00000000-0000-0000-0000-000000000503",
		SystemAccountID: "00000000-0000-0000-0000-000000000504", ExternalReference: "customer-wire-2026-501",
		EvidenceReference: "evidence://customer/wire/2026-501", IdempotencyKey: "funding-request-key-000501",
		CorrelationID: "00000000-0000-0000-0000-000000000505", Amount: amount,
	}, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestProductionFundingRequiresIndependentApprovalBeforePosting(t *testing.T) {
	event := requestedEvent(t)
	if err := event.Approve("finance-requester", "approval-self", false, time.Now()); !errors.Is(err, funding.ErrIndependentApprovalRequired) {
		t.Fatalf("self approval error=%v", err)
	}
	if err := event.Post("00000000-0000-0000-0000-000000000506", time.Now()); !errors.Is(err, funding.ErrInvalidTransition) {
		t.Fatalf("posting before approval error=%v", err)
	}
	approvedAt := time.Date(2026, 8, 28, 10, 5, 0, 0, time.UTC)
	if err := event.Approve("finance-approver", "customer evidence verified", false, approvedAt); err != nil {
		t.Fatal(err)
	}
	if event.Status != funding.StatusApproved || event.ApproverSubjectID != "finance-approver" || event.ApprovedAt == nil || !event.ApprovedAt.Equal(approvedAt) {
		t.Fatalf("unexpected approval state: %#v", event)
	}
}

func TestLocalDemoApprovalIsExplicitAndCannotMasqueradeAsProduction(t *testing.T) {
	event := requestedEvent(t)
	if err := event.Approve("finance-requester", "local demo funding", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !event.DemoPolicy || event.Status != funding.StatusApproved {
		t.Fatalf("demo approval was not labeled: %#v", event)
	}
}

func TestPostedFundingIsImmutableAndCorrectionIsAdditive(t *testing.T) {
	event := requestedEvent(t)
	if err := event.Approve("finance-approver", "verified", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	postedAt := time.Date(2026, 8, 28, 10, 10, 0, 0, time.UTC)
	if err := event.Post("00000000-0000-0000-0000-000000000507", postedAt); err != nil {
		t.Fatal(err)
	}
	if err := event.Reject("late rejection", time.Now()); !errors.Is(err, funding.ErrInvalidTransition) {
		t.Fatalf("posted event mutated by rejection: %v", err)
	}
	if err := event.MarkCompensated("00000000-0000-0000-0000-000000000508", time.Now()); err != nil {
		t.Fatal(err)
	}
	if event.Status != funding.StatusCompensated || event.CompensationEventID == "" || event.JournalTransactionID == "" {
		t.Fatalf("compensation did not preserve original evidence: %#v", event)
	}
}

func TestFundingRequestRejectsMissingEvidenceAndNonPositiveAmount(t *testing.T) {
	zero, err := money.New("USD", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = funding.Request(funding.RequestInput{
		ID: "id", TenantID: "tenant", RequesterSubjectID: "actor", DestinationAccountID: "destination",
		SystemAccountID: "system", ExternalReference: "external", IdempotencyKey: "0123456789abcdef", Amount: zero,
	}, time.Now())
	if !errors.Is(err, funding.ErrInvalidFunding) {
		t.Fatalf("invalid request error=%v", err)
	}
}
