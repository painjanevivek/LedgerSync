package contract_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type contractRepository struct {
	command transfers.Command
	result  transfers.Submission
}

func (r *contractRepository) Submit(_ context.Context, command transfers.Command, _ [sha256.Size]byte) (transfers.Submission, error) {
	r.command = command
	return r.result, nil
}

func TestCreateTransferContractReturnsExactPostedOutcome(t *testing.T) {
	repository := &contractRepository{result: transfers.Submission{Result: transfers.Result{
		TransferID:  "6b093f93-24a5-44c8-bb16-4d8b8ff6f0ad",
		Status:      "posted",
		Currency:    "USD",
		AmountMinor: 12550,
		OccurredAt:  "2026-08-18T09:15:00Z",
		MinimumBalanceVersions: map[string]int64{
			"00000000-0000-0000-0000-000000000010": 42,
			"00000000-0000-0000-0000-000000000020": 17,
		},
	}}}
	service, err := transfers.NewService(repository, func() time.Time { return time.Date(2026, 8, 18, 9, 15, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers.NewTransferHandler(service, identity.DevelopmentProvider{SubjectID: "operator-1", TenantID: "00000000-0000-0000-0000-000000000001"})
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{
"source_account_id":"00000000-0000-0000-0000-000000000010",
"destination_account_id":"00000000-0000-0000-0000-000000000020",
"amount":"125.50","currency":"USD"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", "transfer-contract-key-001")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.command.Amount.Minor() != 12550 || repository.command.Amount.Currency().Code != "USD" {
		t.Fatalf("handler did not parse exact money: %#v", repository.command.Amount)
	}
	if repository.command.IdempotencyKey != "transfer-contract-key-001" {
		t.Fatalf("idempotency key = %q", repository.command.IdempotencyKey)
	}
	var body transfers.Result
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TransferID != repository.result.Result.TransferID || body.Status != "posted" || body.AmountMinor != 12550 {
		t.Fatalf("unexpected transfer result: %#v", body)
	}
}

func TestCreateTransferContractRejectsMalformedInputWithoutCallingService(t *testing.T) {
	repository := &contractRepository{}
	service, err := transfers.NewService(repository, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers.NewTransferHandler(service, identity.DevelopmentProvider{SubjectID: "operator-1", TenantID: "tenant-1"})
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{"source_account_id":"a","destination_account_id":"b","amount":"1.999","currency":"USD"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", "transfer-contract-key-002")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.command.TenantID != "" {
		t.Fatalf("service received invalid request: %#v", repository.command)
	}
}
