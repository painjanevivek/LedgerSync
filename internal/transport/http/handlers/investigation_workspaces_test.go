package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type workspaceRepositoryStub struct {
	*investigationRepositoryStub
	create  investigation.WorkspaceCreate
	handoff investigation.WorkspaceHandoff
	status  investigation.WorkspaceStatusChange
	access  investigation.SearchAccess
	err     error
}

func (r *workspaceRepositoryStub) ListWorkspaces(_ context.Context, _, _ string, access investigation.SearchAccess) (investigation.WorkspacePage, error) {
	r.called, r.access = true, access
	return investigation.WorkspacePage{Investigations: []investigation.WorkspaceSummary{}}, r.err
}
func (r *workspaceRepositoryStub) CreateWorkspace(_ context.Context, command investigation.WorkspaceCreate) (investigation.Workspace, error) {
	r.called, r.create = true, command
	return investigation.Workspace{WorkspaceSummary: investigation.WorkspaceSummary{ID: "11111111-1111-4111-8111-111111111111", Title: command.Title, Status: "open", Version: "1"}}, r.err
}
func (r *workspaceRepositoryStub) GetWorkspace(_ context.Context, _, _, id string, access investigation.SearchAccess) (investigation.Workspace, error) {
	r.called, r.access = true, access
	return investigation.Workspace{WorkspaceSummary: investigation.WorkspaceSummary{ID: id, Title: "Review", Status: "open", Version: "1"}}, r.err
}
func (r *workspaceRepositoryStub) HandoffWorkspace(_ context.Context, command investigation.WorkspaceHandoff) (investigation.WorkspaceReceipt, error) {
	r.called, r.handoff = true, command
	return investigation.WorkspaceReceipt{InvestigationID: command.InvestigationID, Outcome: "handed_off", Version: "2"}, r.err
}
func (r *workspaceRepositoryStub) ChangeWorkspaceStatus(_ context.Context, command investigation.WorkspaceStatusChange) (investigation.WorkspaceReceipt, error) {
	r.called, r.status = true, command
	return investigation.WorkspaceReceipt{InvestigationID: command.InvestigationID, Outcome: command.TargetStatus, Version: "2"}, r.err
}

func workspaceHandler(repository *workspaceRepositoryStub, scopes ...string) http.Handler {
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = struct{}{}
	}
	provider := fixedInvestigationPrincipal{principal: identity.Principal{SubjectID: "operator-1", TenantID: "tenant", Roles: map[string]struct{}{"tenant:operator": {}}, Scopes: scopeSet}}
	handler := NewInvestigationHandler(repository, provider)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/investigation/workspaces", handler.Workspaces)
	mux.HandleFunc("POST /api/investigation/workspaces", handler.CreateWorkspace)
	mux.HandleFunc("GET /api/investigation/workspaces/{investigationId}", handler.Workspace)
	mux.HandleFunc("POST /api/investigation/workspaces/{investigationId}/handoff", handler.HandoffWorkspace)
	mux.HandleFunc("POST /api/investigation/workspaces/{investigationId}/close", handler.CloseWorkspace)
	mux.HandleFunc("POST /api/investigation/workspaces/{investigationId}/reopen", handler.ReopenWorkspace)
	return middleware.Correlation(mux)
}

func TestWorkspaceCreateUsesAuthorizedServerCapturedRoot(t *testing.T) {
	repository := &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	body := `{"title":"Delayed transfer review","taxonomy":"transfer_delivery","query_context":{"kind":"immutable_id","record_type":"transfer","value":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"},"root_record":{"record_type":"transfer","record_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	workspaceHandler(repository, "investigation:read", "investigation:write", "transfers:read").ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !repository.called || !repository.create.Access.Transfers || repository.create.CorrelationID == "" || response.Header().Get("Location") == "" {
		t.Fatalf("status=%d command=%#v body=%s", response.Code, repository.create, response.Body.String())
	}
}

func TestWorkspaceRejectsCopiedEvidenceAndRequiresWriteScope(t *testing.T) {
	valid := `{"title":"Review","taxonomy":"other","query_context":{"kind":"immutable_id","record_type":"transfer","value":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"},"root_record":{"record_type":"transfer","record_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}}`
	for _, body := range []string{
		strings.TrimSuffix(valid, "}") + `,"notes":"copied balance 100"}`,
		strings.Replace(valid, `"record_type":"transfer","record_id"`, `"record_type":"account","record_id"`, 1),
	} {
		repository := &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
		request := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		workspaceHandler(repository, "investigation:read", "investigation:write", "transfers:read").ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || repository.called {
			t.Fatalf("accepted body=%s status=%d", body, response.Code)
		}
	}
	repository := &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	request := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces", strings.NewReader(valid))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	workspaceHandler(repository, "investigation:read", "transfers:read").ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || repository.called {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestWorkspaceHandoffCloseAndReopenUseOptimisticVersion(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	repository := &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	handoff := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces/"+id+"/handoff", strings.NewReader(`{"expected_version":"3","target_subject_id":"operator-2"}`))
	handoff.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	workspaceHandler(repository, "investigation:read", "investigation:write", "transfers:read").ServeHTTP(response, handoff)
	if response.Code != http.StatusOK || repository.handoff.ExpectedVersion != 3 || repository.handoff.TargetSubjectID != "operator-2" || !repository.handoff.Access.Transfers {
		t.Fatalf("status=%d command=%#v body=%s", response.Code, repository.handoff, response.Body.String())
	}
	for route, want := range map[string]string{"close": "closed", "reopen": "open"} {
		repository = &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
		request := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces/"+id+"/"+route, strings.NewReader(`{"expected_version":"4"}`))
		request.Header.Set("Content-Type", "application/json")
		response = httptest.NewRecorder()
		workspaceHandler(repository, "investigation:read", "investigation:write", "transfers:read").ServeHTTP(response, request)
		if response.Code != http.StatusOK || repository.status.TargetStatus != want || repository.status.ExpectedVersion != 4 || !repository.status.Access.Transfers {
			t.Fatalf("route=%s status=%d command=%#v", route, response.Code, repository.status)
		}
	}
}
