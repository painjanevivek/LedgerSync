package handlers

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

func TestExhaustedSerializableConflictRequestsSafeSameKeyRetry(t *testing.T) {
	err := publicTransferError(errors.New("commit failed: SQLSTATE 40001"))
	var public *httptransport.PublicError
	if !errors.As(err, &public) {
		t.Fatalf("retryable transaction conflict was not mapped to a public availability result: %v", err)
	}
	if public.Status != http.StatusServiceUnavailable || public.Code != "transaction_conflict_retryable" {
		t.Fatalf("unexpected transaction conflict contract: %+v", public)
	}
}

type scopeTestProvider struct{}

func (scopeTestProvider) Authenticate(context.Context, string) (identity.Principal, error) {
	return identity.Principal{SubjectID: "operator", TenantID: "tenant", Scopes: map[string]struct{}{"transfers:read": {}}}, nil
}

type scopeTestRepository struct {
	called bool
}

func (repository *scopeTestRepository) Submit(context.Context, transfers.Command, [sha256.Size]byte) (transfers.Submission, error) {
	repository.called = true
	return transfers.Submission{}, nil
}

func TestMissingWriteScopeIsDeniedBeforeFinancialInputOrObjectDisclosure(t *testing.T) {
	repository := &scopeTestRepository{}
	service, err := transfers.NewService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewTransferHandler(service, scopeTestProvider{})
	body := `{"source_account_id":"sensitive-source","destination_account_id":"sensitive-destination","amount":"1.00","currency":"INR"}`
	request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer read-only")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repository.called {
		t.Fatal("scope-denied request reached the financial repository")
	}
	if strings.Contains(response.Body.String(), "sensitive-source") || strings.Contains(response.Body.String(), "sensitive-destination") {
		t.Fatalf("scope denial disclosed object identifiers: %s", response.Body.String())
	}
}
