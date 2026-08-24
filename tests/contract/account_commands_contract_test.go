package contract_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type accountCommandContractRepository struct {
	mu          sync.Mutex
	fingerprint *[sha256.Size]byte
	result      accounts.CommandResult
	status      accounts.ChangeAccountStatusCommand
}

func (r *accountCommandContractRepository) Create(_ context.Context, _ accounts.CreateAccountCommand, fingerprint [sha256.Size]byte) (accounts.CommandSubmission, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fingerprint == nil {
		r.fingerprint = new([sha256.Size]byte)
		*r.fingerprint = fingerprint
		return accounts.CommandSubmission{Result: r.result}, nil
	}
	if *r.fingerprint != fingerprint {
		return accounts.CommandSubmission{}, accounts.ErrIdempotencyConflict
	}
	return accounts.CommandSubmission{Result: r.result, Replayed: true}, nil
}

func (*accountCommandContractRepository) UpdateMetadata(context.Context, accounts.UpdateAccountMetadataCommand, [sha256.Size]byte) (accounts.CommandSubmission, error) {
	return accounts.CommandSubmission{}, nil
}

func (r *accountCommandContractRepository) ChangeStatus(_ context.Context, command accounts.ChangeAccountStatusCommand, _ [sha256.Size]byte) (accounts.CommandSubmission, error) {
	r.status = command
	return accounts.CommandSubmission{Result: r.result}, nil
}

func TestLifecyclePatchContractRequiresExclusiveBoundedReason(t *testing.T) {
	repository := &accountCommandContractRepository{result: accounts.CommandResult{AccountID: "70000000-0000-4000-8000-000000000001", Version: "2"}}
	service, err := accounts.NewCommandService(repository, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	commandHandler := handlers.NewAccountCommandHandler(service, identity.DevelopmentProvider{SubjectID: "demo-operator", TenantID: "00000000-0000-4000-8000-000000000001", Scopes: []string{"accounts:write"}})
	router := http.NewServeMux()
	router.HandleFunc("PATCH /api/accounts/{accountID}", commandHandler.Patch)
	handler := middleware.Correlation(router)

	for name, body := range map[string]string{
		"missing reason":       `{"expected_version":"1","target_status":"frozen"}`,
		"metadata with reason": `{"expected_version":"1","display_name":"Ops","external_reference":"ops-inr","category":"operating","reason":"wrong family"}`,
	} {
		response := executeAccountPatchContractRequest(handler, body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", name, response.Code, response.Body.String())
		}
	}
	valid := executeAccountPatchContractRequest(handler, `{"expected_version":"1","target_status":"frozen","reason":"  नियमित समीक्षा  "}`)
	if valid.Code != http.StatusOK || repository.status.Reason != "नियमित समीक्षा" {
		t.Fatalf("status=%d reason=%q body=%s", valid.Code, repository.status.Reason, valid.Body.String())
	}
}

func executeAccountPatchContractRequest(handler http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPatch, "/api/accounts/70000000-0000-4000-8000-000000000001", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "account-patch-contract-001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAccountCreateContractPreservesLostResponseRetryAndChangedIntentConflict(t *testing.T) {
	repository := &accountCommandContractRepository{result: accounts.CommandResult{
		AccountID: "70000000-0000-4000-8000-000000000001", TenantID: "00000000-0000-4000-8000-000000000001",
		Currency: "INR", Status: "active", DisplayName: "India Operations", Reference: "india-operations", Category: "operating",
		Version: "1", AvailableMinor: "0", LedgerMinor: "0", CreatedAt: "2026-08-25T10:00:00Z", UpdatedAt: "2026-08-25T10:00:00Z",
	}}
	service, err := accounts.NewCommandService(repository, func() time.Time { return time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	commandHandler := handlers.NewAccountCommandHandler(service, identity.DevelopmentProvider{SubjectID: "demo-operator", TenantID: repository.result.TenantID, Scopes: []string{"accounts:write"}})
	router := http.NewServeMux()
	router.HandleFunc("POST /api/accounts", commandHandler.Create)
	handler := middleware.Correlation(router)

	originalBody := `{"display_name":"India Operations","external_reference":"india-operations","category":"operating","currency":"INR"}`
	original := executeAccountCreateContractRequest(handler, originalBody)
	if original.Code != http.StatusCreated || original.Header().Get("Idempotent-Replay") != "" || original.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("original status=%d headers=%v body=%s", original.Code, original.Header(), original.Body.String())
	}

	// The first response may have been lost by the caller. Retrying the exact
	// intent and key must return the persisted identity without creating again.
	replay := executeAccountCreateContractRequest(handler, originalBody)
	if replay.Code != http.StatusCreated || replay.Header().Get("Idempotent-Replay") != "true" || replay.Body.String() != original.Body.String() {
		t.Fatalf("replay status=%d headers=%v body=%s original=%s", replay.Code, replay.Header(), replay.Body.String(), original.Body.String())
	}
	if !strings.Contains(replay.Body.String(), `"account_version":"1"`) || strings.Contains(replay.Body.String(), `"version":`) || strings.Contains(replay.Body.String(), `"reference":`) {
		t.Fatalf("response does not preserve account command transport fields: %s", replay.Body.String())
	}

	changed := executeAccountCreateContractRequest(handler, `{"display_name":"Changed intent","external_reference":"india-operations","category":"operating","currency":"INR"}`)
	if changed.Code != http.StatusConflict || !strings.Contains(changed.Body.String(), `"code":"idempotency_conflict"`) {
		t.Fatalf("changed-intent status=%d body=%s", changed.Code, changed.Body.String())
	}
}

func executeAccountCreateContractRequest(handler http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "account-contract-key-0001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
