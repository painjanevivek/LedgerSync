package handlers

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const (
	fundingTestAccountID = "10000000-0000-4000-8000-000000000001"
	fundingTestEventID   = "30000000-0000-4000-8000-000000000001"
)

type fundingHandlerRepository struct {
	request       appfunding.RequestCommand
	decision      appfunding.DecisionCommand
	demoApproval  bool
	requestCalls  int
	decisionCalls int
}

func (r *fundingHandlerRepository) Request(_ context.Context, command appfunding.RequestCommand, _ [sha256.Size]byte) (appfunding.Submission, error) {
	r.request, r.requestCalls = command, r.requestCalls+1
	return appfunding.Submission{Event: appfunding.Event{FundingEventID: fundingTestEventID, Status: "requested", AmountMinor: strconv.FormatInt(command.Amount.Minor(), 10), Currency: command.Amount.Currency().Code}}, nil
}

func (r *fundingHandlerRepository) Approve(_ context.Context, command appfunding.DecisionCommand, demo bool) (appfunding.Event, error) {
	r.decision, r.demoApproval, r.decisionCalls = command, demo, r.decisionCalls+1
	return appfunding.Event{FundingEventID: command.FundingEventID, Status: "approved", DemoPolicy: demo}, nil
}

func (r *fundingHandlerRepository) Reject(context.Context, appfunding.DecisionCommand) (appfunding.Event, error) {
	return appfunding.Event{}, nil
}

func (r *fundingHandlerRepository) Post(context.Context, appfunding.ActionCommand) (appfunding.Submission, error) {
	return appfunding.Submission{}, nil
}

func (r *fundingHandlerRepository) Compensate(context.Context, appfunding.CompensationCommand, [sha256.Size]byte) (appfunding.Submission, error) {
	return appfunding.Submission{}, nil
}

func (r *fundingHandlerRepository) Get(context.Context, string, string, string) (appfunding.Event, error) {
	return appfunding.Event{}, nil
}

func (r *fundingHandlerRepository) List(context.Context, string, string, appfunding.Query) (appfunding.Page, error) {
	return appfunding.Page{}, nil
}

func (r *fundingHandlerRepository) Reconcile(context.Context, string, string, string) (appfunding.Reconciliation, error) {
	return appfunding.Reconciliation{}, nil
}

func TestFundingRequestRequiresExactMinorUnitsAndWriteScope(t *testing.T) {
	repository := &fundingHandlerRepository{}
	handler := newTestFundingHandler(t, repository, []string{"funding:write"})
	request := httptest.NewRequest(http.MethodPost, "/api/funding-requests", strings.NewReader(`{"destination_account_id":"`+fundingTestAccountID+`","amount_minor":"1250","currency":"USD","external_reference":"wire-1","evidence_reference":"evidence://wire-1"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", strings.Repeat("i", 20))
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.Request)).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	if repository.requestCalls != 1 || repository.request.TenantID != "tenant-1" || repository.request.ActorSubjectID != "finance-1" || repository.request.Amount.Minor() != 1250 {
		t.Fatalf("request=%#v calls=%d", repository.request, repository.requestCalls)
	}

	deniedRepository := &fundingHandlerRepository{}
	denied := newTestFundingHandler(t, deniedRepository, []string{"funding:read"})
	deniedRequest := httptest.NewRequest(http.MethodPost, "/api/funding-requests", strings.NewReader(`{}`))
	deniedRequest.Header.Set("Authorization", "Bearer development-local-only")
	deniedResponse := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(denied.Request)).ServeHTTP(deniedResponse, deniedRequest)
	if deniedResponse.Code != http.StatusForbidden || deniedRepository.requestCalls != 0 {
		t.Fatalf("denied status=%d calls=%d", deniedResponse.Code, deniedRepository.requestCalls)
	}
}

func TestFundingApprovalPreservesExplicitLocalDemoLabel(t *testing.T) {
	repository := &fundingHandlerRepository{}
	handler := newTestFundingHandler(t, repository, []string{"funding:approve"})
	request := httptest.NewRequest(http.MethodPost, "/api/funding-events/"+fundingTestEventID+"/approve", strings.NewReader(`{"reason":"evidence independently verified"}`))
	request.SetPathValue("fundingEventId", fundingTestEventID)
	request.Header.Set("Authorization", "Bearer development-local-only")
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.Approve)).ServeHTTP(response, request)
	if response.Code != http.StatusOK || repository.decisionCalls != 1 || !repository.demoApproval || repository.decision.FundingEventID != fundingTestEventID {
		t.Fatalf("status=%d decision=%#v demo=%t body=%s", response.Code, repository.decision, repository.demoApproval, response.Body.String())
	}
}

func TestFundingRequestRejectsAmbiguousJSONBeforeApplication(t *testing.T) {
	repository := &fundingHandlerRepository{}
	handler := newTestFundingHandler(t, repository, []string{"funding:write"})
	request := httptest.NewRequest(http.MethodPost, "/api/funding-requests", strings.NewReader(`{"destination_account_id":"account-1","amount_minor":"1.25","currency":"USD","external_reference":"wire-1","evidence_reference":"evidence://wire-1","unexpected":true}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Idempotency-Key", strings.Repeat("i", 20))
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.Request)).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || repository.requestCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, repository.requestCalls, response.Body.String())
	}
}

func newTestFundingHandler(t *testing.T, repository appfunding.Repository, scopes []string) *FundingHandler {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	service, err := appfunding.NewService(repository, appfunding.PolicyLocalDemoSingleOperator, clock)
	if err != nil {
		t.Fatal(err)
	}
	return NewFundingHandler(service, identity.DevelopmentProvider{SubjectID: "finance-1", TenantID: "tenant-1", Scopes: scopes})
}
