package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
)

func TestAccountReadDTOKeepsConfigurationAndBalanceVersionsDistinct(t *testing.T) {
	response := mapAccountResponse(accounts.Summary{
		AccountID: "account-1", AccountVersion: 7,
		Balance: accounts.Balance{Version: 42},
	})
	if response.AccountVersion != "7" || response.Version != "42" {
		t.Fatalf("account_version=%q balance version=%q", response.AccountVersion, response.Version)
	}
}

func TestAccountDetailAuditResponseAddsOnlyPresentLifecycleReason(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAccountResponse(recorder, accounts.Summary{AuditContext: []accounts.AuditEvent{
		{EventID: "event-1", EventType: "account.status_changed", ActorSubjectID: "operator-1", Outcome: "succeeded", CorrelationID: "correlation-1", Reason: "Quarter-end review", OccurredAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)},
		{EventID: "event-2", EventType: "account.created", ActorSubjectID: "operator-1", Outcome: "succeeded", CorrelationID: "correlation-2", OccurredAt: time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)},
	}})

	var response struct {
		AuditContext []map[string]any `json:"audit_context"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.AuditContext) != 2 {
		t.Fatalf("audit event count=%d, want 2", len(response.AuditContext))
	}
	if got := response.AuditContext[0]["reason"]; got != "Quarter-end review" {
		t.Fatalf("lifecycle reason=%v, want projected reason", got)
	}
	if _, present := response.AuditContext[1]["reason"]; present {
		t.Fatal("empty lifecycle reason must remain an additive omitted field")
	}
}
