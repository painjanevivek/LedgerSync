package db

import (
	"testing"
	"time"

	appapprovals "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/approvals"
)

func TestApprovalCursorRoundTripsStableOldestFirstPosition(t *testing.T) {
	expected := approvalCursor{
		RequestedAt: time.Date(2026, 8, 31, 12, 0, 0, 123, time.UTC),
		RecordID:    "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		Domain:      appapprovals.DomainCorrection,
		Actionable:  true,
	}
	actual, err := decodeApprovalCursor(encodeApprovalCursor(expected))
	if err != nil {
		t.Fatal(err)
	}
	if !actual.RequestedAt.Equal(expected.RequestedAt) || actual.RecordID != expected.RecordID || actual.Domain != expected.Domain || actual.Actionable != expected.Actionable {
		t.Fatalf("cursor=%#v", actual)
	}
	if _, err := decodeApprovalCursor("not-a-valid-cursor"); err == nil {
		t.Fatal("invalid cursor must fail closed")
	}
}

func TestApprovalItemDecorationPrioritizesSafeStops(t *testing.T) {
	tests := []struct {
		name      string
		item      appapprovals.Item
		stepUp    bool
		satisfied bool
		action    string
		status    appapprovals.StepUpStatus
	}{
		{"self approval", appapprovals.Item{Domain: appapprovals.DomainFunding, SelfApprovalBlocked: true, EvidenceComplete: true, ActionableByMe: true}, false, false, "wait_for_independent_approver", appapprovals.StepUpNotRequired},
		{"missing evidence", appapprovals.Item{Domain: appapprovals.DomainCorrection, EvidenceComplete: false, ActionableByMe: true}, true, false, "complete_evidence", appapprovals.StepUpRequired},
		{"step up", appapprovals.Item{Domain: appapprovals.DomainCorrection, EvidenceComplete: true, ActionableByMe: true}, true, false, "reauthenticate", appapprovals.StepUpRequired},
		{"review", appapprovals.Item{Domain: appapprovals.DomainCorrection, EvidenceComplete: true, ActionableByMe: true}, true, true, "review_decision", appapprovals.StepUpSatisfied},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			decorateApprovalItem(&testCase.item, testCase.stepUp, testCase.satisfied)
			if testCase.item.SafeNextAction != testCase.action || testCase.item.StepUpStatus != testCase.status {
				t.Fatalf("item=%#v", testCase.item)
			}
		})
	}
}
