package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type reconciliationCommandRepositoryStub struct {
	command    reconciliation.RunCommand
	submission reconciliation.CommandSubmission
	err        error
}

func (r *reconciliationCommandRepositoryStub) RunCommand(_ context.Context, command reconciliation.RunCommand, _ [sha256.Size]byte) (reconciliation.CommandSubmission, error) {
	r.command = command
	return r.submission, r.err
}

func reconciliationCommandTestHandler(t *testing.T, repository *reconciliationCommandRepositoryStub, scopes ...string) http.Handler {
	t.Helper()
	service, err := reconciliation.NewCommandService(repository, func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	handler := NewReconciliationCommandHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: scopes})
	router := http.NewServeMux()
	router.HandleFunc("POST /api/reconciliation/runs", handler.Run)
	return middleware.Correlation(router)
}

func reconciliationCommandRequest(key string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/reconciliation/runs", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	return request
}

func TestReconciliationCommandReturnsExactDTOAndReplay(t *testing.T) {
	started := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repository := &reconciliationCommandRepositoryStub{submission: reconciliation.CommandSubmission{Replayed: true, Result: reconciliation.Result{
		ID: "00000000-0000-4000-8000-000000000001", TenantID: "tenant", CorrelationID: "00000000-0000-4000-8000-000000000099", Status: reconciliation.StatusMismatch,
		Scope: "tenant_all_accounts", LedgerWatermark: "1:2:", ApplicationVersion: "development", SchemaVersion: "14", CheckedAccountCount: 2, PostingCount: 4, MismatchCount: 1, StartedAt: started, CompletedAt: started.Add(time.Second),
	}}}
	response := httptest.NewRecorder()
	reconciliationCommandTestHandler(t, repository, "reconciliation:write").ServeHTTP(response, reconciliationCommandRequest("reconciliation-key-0001"))
	if response.Code != http.StatusCreated || response.Header().Get("Idempotent-Replay") != "true" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "mismatch" || body["checked_account_count"] != "2" || body["mismatch_count"] != "1" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if repository.command.TenantID != "tenant" || repository.command.ActorSubjectID != "operator" || repository.command.CorrelationID == "" {
		t.Fatalf("unexpected command: %#v", repository.command)
	}
}

func TestReconciliationCommandRequiresWriteScopeAndStrictEmptyObject(t *testing.T) {
	bodyRequest := httptest.NewRequest(http.MethodPost, "/api/reconciliation/runs", strings.NewReader(`{}`))
	bodyRequest.Header.Set("Authorization", "Bearer development-local-only")
	bodyRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
	bodyRequest.Header.Set("Idempotency-Key", "reconciliation-key-0001")
	nonemptyRequest := httptest.NewRequest(http.MethodPost, "/api/reconciliation/runs", strings.NewReader(`{"unexpected":true}`))
	nonemptyRequest.Header.Set("Authorization", "Bearer development-local-only")
	nonemptyRequest.Header.Set("Content-Type", "application/json")
	nonemptyRequest.Header.Set("Idempotency-Key", "reconciliation-key-0002")
	for name, testCase := range map[string]struct {
		handler http.Handler
		request *http.Request
		status  int
	}{
		"read scope":    {reconciliationCommandTestHandler(t, &reconciliationCommandRepositoryStub{}, "reconciliation:read"), reconciliationCommandRequest("reconciliation-key-0001"), http.StatusForbidden},
		"empty object":  {reconciliationCommandTestHandler(t, &reconciliationCommandRepositoryStub{}, "reconciliation:write"), bodyRequest, http.StatusCreated},
		"unknown field": {reconciliationCommandTestHandler(t, &reconciliationCommandRepositoryStub{}, "reconciliation:write"), nonemptyRequest, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.handler.ServeHTTP(response, testCase.request)
			if response.Code != testCase.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReconciliationCommandMapsStableDenialAndUnknownOutcome(t *testing.T) {
	for name, testCase := range map[string]struct {
		repository *reconciliationCommandRepositoryStub
		status     int
		code       string
	}{
		"already running": {&reconciliationCommandRepositoryStub{submission: reconciliation.CommandSubmission{Denial: "already_running", Replayed: true, ActiveRunID: "00000000-0000-4000-8000-000000000077"}}, http.StatusConflict, "reconciliation_already_running"},
		"in progress":     {&reconciliationCommandRepositoryStub{err: reconciliation.ErrCommandInProgress}, http.StatusConflict, "request_in_progress"},
		"unknown":         {&reconciliationCommandRepositoryStub{err: context.DeadlineExceeded}, http.StatusGatewayTimeout, "response_unknown"},
		"commit unknown":  {&reconciliationCommandRepositoryStub{err: reconciliation.ErrResponseUnknown}, http.StatusGatewayTimeout, "response_unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			reconciliationCommandTestHandler(t, testCase.repository, "reconciliation:write").ServeHTTP(response, reconciliationCommandRequest("reconciliation-key-0001"))
			if response.Code != testCase.status || !containsJSONCode(response.Body.Bytes(), testCase.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func containsJSONCode(body []byte, code string) bool {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	return json.Unmarshal(body, &envelope) == nil && envelope.Error.Code == code
}
