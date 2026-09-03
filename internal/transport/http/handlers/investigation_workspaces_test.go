package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type workspaceRepositoryStub struct {
	*investigationRepositoryStub
	workspace investigation.Workspace
	create    investigation.WorkspaceCreate
	handoff   investigation.WorkspaceHandoff
	status    investigation.WorkspaceStatusChange
	access    investigation.SearchAccess
	err       error
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
	if r.workspace.ID != "" {
		return r.workspace, r.err
	}
	return investigation.Workspace{WorkspaceSummary: investigation.WorkspaceSummary{ID: id, Title: "Review", Status: "open", Version: "1"}}, r.err
}

func workspaceBundleHandler(repository *workspaceRepositoryStub, audit *exportAuditStub, scopes ...string) http.Handler {
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = struct{}{}
	}
	provider := fixedInvestigationPrincipal{principal: identity.Principal{SubjectID: "operator-1", TenantID: "tenant", Roles: map[string]struct{}{"tenant:operator": {}}, Scopes: scopeSet}}
	handler := NewInvestigationHandler(repository, provider).WithAuditRecorder(audit)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/investigation/workspaces/{investigationId}/evidence-bundle", handler.WorkspaceEvidenceBundle)
	return middleware.Correlation(mux)
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

func TestWorkspaceEvidenceBundleRequiresExactVersionAndAudit(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	id := "11111111-1111-4111-8111-111111111111"
	repository := &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}, workspace: investigation.Workspace{WorkspaceSummary: investigation.WorkspaceSummary{ID: id, Status: "open", Taxonomy: "other", Version: "4"}, HistoricalContext: investigation.WorkspaceHistoricalContext{References: []investigation.WorkspaceReference{{RelationshipType: "workspace_root", RecordType: "transfer", RecordID: "22222222-2222-4222-8222-222222222222", CapturedAt: now}}}}}
	audit := &exportAuditStub{}
	request := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces/"+id+"/evidence-bundle", strings.NewReader(`{"expected_version":"4"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	workspaceBundleHandler(repository, audit, "investigation:read", "exports:read", "transfers:read").ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/zip" || response.Header().Get("X-LedgerSync-Bundle-SHA256") == "" || len(audit.events) != 1 {
		t.Fatalf("status=%d headers=%v audit=%#v body=%s", response.Code, response.Header(), audit.events, response.Body.String())
	}
	if _, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len())); err != nil {
		t.Fatal(err)
	}
	if audit.events[0].EventType != "investigation.evidence_bundle_generated" || audit.events[0].TargetID != id {
		t.Fatalf("unexpected audit: %#v", audit.events[0])
	}
}

func TestWorkspaceEvidenceBundleFailsClosedForScopeVersionAndAudit(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	base := investigation.Workspace{WorkspaceSummary: investigation.WorkspaceSummary{ID: id, Status: "open", Taxonomy: "other", Version: "4"}}
	for name, tc := range map[string]struct {
		scopes   []string
		version  string
		auditErr error
		want     int
	}{
		"scope":   {[]string{"investigation:read", "transfers:read"}, "4", nil, http.StatusForbidden},
		"version": {[]string{"investigation:read", "exports:read", "transfers:read"}, "3", nil, http.StatusConflict},
		"audit":   {[]string{"investigation:read", "exports:read", "transfers:read"}, "4", errors.New("audit unavailable"), http.StatusServiceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			repository := &workspaceRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}, workspace: base}
			audit := &exportAuditStub{err: tc.auditErr}
			request := httptest.NewRequest(http.MethodPost, "/api/investigation/workspaces/"+id+"/evidence-bundle", strings.NewReader(`{"expected_version":"`+tc.version+`"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			workspaceBundleHandler(repository, audit, tc.scopes...).ServeHTTP(response, request)
			if response.Code != tc.want || response.Header().Get("Content-Disposition") != "" {
				t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
		})
	}
}
