package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type investigationRepositoryStub struct {
	called bool
	filter investigation.TransferFilter
}

func (r *investigationRepositoryStub) ListTransfers(_ context.Context, _ string, filter investigation.TransferFilter) ([]investigation.TransferSummary, string, error) {
	r.called = true
	r.filter = filter
	return nil, "", nil
}

func TestInvestigationTransferFiltersAreStrictBoundedAndServerSide(t *testing.T) {
	principal := fixedInvestigationPrincipal{principal: identity.Principal{
		SubjectID: "operator", TenantID: "tenant",
		Roles: map[string]struct{}{"tenant:operator": {}}, Scopes: map[string]struct{}{"transfers:read": {}},
	}}
	repository := &investigationRepositoryStub{}
	handler := NewInvestigationHandler(repository, principal)
	request := httptest.NewRequest(http.MethodGet, "/api/transfers?status=posted&q=ABC-def&limit=20", nil)
	response := httptest.NewRecorder()
	handler.Transfers(response, request)
	if response.Code != http.StatusOK || !repository.called || repository.filter.Status != "posted" || repository.filter.Query != "ABC-def" || repository.filter.Limit != 20 {
		t.Fatalf("status=%d called=%v filter=%#v body=%s", response.Code, repository.called, repository.filter, response.Body.String())
	}

	invalid := []string{
		"/api/transfers?status=settled",
		"/api/transfers?q=abc%25",
		"/api/transfers?q=abc&q=def",
		"/api/transfers?loadedPageOnly=true",
		"/api/transfers?accountId=not-a-uuid",
		"/api/transfers?from=2026-08-26T00%3A00%3A00Z&to=2026-08-25T00%3A00%3A00Z",
	}
	for _, target := range invalid {
		t.Run(target, func(t *testing.T) {
			repository := &investigationRepositoryStub{}
			response := httptest.NewRecorder()
			handler := NewInvestigationHandler(repository, principal)
			handler.Transfers(response, httptest.NewRequest(http.MethodGet, target, nil))
			if response.Code != http.StatusBadRequest || repository.called {
				t.Fatalf("status=%d called=%v body=%s", response.Code, repository.called, response.Body.String())
			}
		})
	}
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
