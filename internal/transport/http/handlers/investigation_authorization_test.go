package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type investigationRepositoryStub struct{ called bool }

func (r *investigationRepositoryStub) ListTransfers(context.Context, string, investigation.TransferFilter) ([]investigation.TransferSummary, string, error) {
	r.called = true
	return nil, "", nil
}
func (r *investigationRepositoryStub) GetTransfer(context.Context, string, string) (investigation.TransferDetail, error) {
	r.called = true
	return investigation.TransferDetail{}, nil
}
func (r *investigationRepositoryStub) ListReconciliationRuns(context.Context, string, string, int) ([]investigation.ReconciliationRun, string, error) {
	r.called = true
	return nil, "", nil
}
func (r *investigationRepositoryStub) GetReconciliationRun(context.Context, string, string) (investigation.ReconciliationRun, error) {
	r.called = true
	return investigation.ReconciliationRun{}, nil
}

type fixedInvestigationPrincipal struct{ principal identity.Principal }

func (p fixedInvestigationPrincipal) Authenticate(context.Context, string) (identity.Principal, error) {
	return p.principal, nil
}

func TestInvestigationRequiresTenantWideOperatorRole(t *testing.T) {
	repository := &investigationRepositoryStub{}
	provider := fixedInvestigationPrincipal{principal: identity.Principal{
		SubjectID: "narrow-reader",
		TenantID:  "tenant",
		Scopes:    map[string]struct{}{"transfers:read": {}},
	}}
	handler := NewInvestigationHandler(repository, provider)
	request := httptest.NewRequest(http.MethodGet, "/api/transfers", nil)
	response := httptest.NewRecorder()

	handler.Transfers(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repository.called {
		t.Fatal("tenant-wide investigation repository was called for a narrow reader")
	}
}

func TestInvestigationAllowsServerOwnedOperatorRole(t *testing.T) {
	repository := &investigationRepositoryStub{}
	provider := fixedInvestigationPrincipal{principal: identity.Principal{
		SubjectID: "operator",
		TenantID:  "tenant",
		Roles:     map[string]struct{}{"tenant:operator": {}},
		Scopes:    map[string]struct{}{"transfers:read": {}},
	}}
	handler := NewInvestigationHandler(repository, provider)
	request := httptest.NewRequest(http.MethodGet, "/api/transfers", nil)
	response := httptest.NewRecorder()

	handler.Transfers(response, request)

	if response.Code != http.StatusOK || !repository.called {
		t.Fatalf("operator status=%d called=%v body=%s", response.Code, repository.called, response.Body.String())
	}
}
