package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type savedViewRepositoryStub struct {
	*investigationRepositoryStub
	create      investigation.SavedViewCreate
	rename      investigation.SavedViewRename
	delete      investigation.SavedViewDelete
	access      investigation.SavedViewAccess
	mutationErr error
}

func (r *savedViewRepositoryStub) ListSavedViews(_ context.Context, _, _ string, access investigation.SavedViewAccess) (investigation.SavedViewPage, error) {
	r.called, r.access = true, access
	return investigation.SavedViewPage{Views: []investigation.SavedView{}}, nil
}
func (r *savedViewRepositoryStub) CreateSavedView(_ context.Context, command investigation.SavedViewCreate) (investigation.SavedView, error) {
	r.called, r.create = true, command
	return investigation.SavedView{ID: "11111111-1111-4111-8111-111111111111", Name: command.Name}, r.mutationErr
}
func (r *savedViewRepositoryStub) RenameSavedView(_ context.Context, command investigation.SavedViewRename) (investigation.SavedView, error) {
	r.called, r.rename = true, command
	return investigation.SavedView{ID: command.SavedViewID, Name: command.Name}, r.mutationErr
}
func (r *savedViewRepositoryStub) DeleteSavedView(_ context.Context, command investigation.SavedViewDelete) error {
	r.called, r.delete = true, command
	return r.mutationErr
}

func savedViewHandler(repository *savedViewRepositoryStub, scopes ...string) http.Handler {
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = struct{}{}
	}
	provider := fixedInvestigationPrincipal{principal: identity.Principal{SubjectID: "operator", TenantID: "tenant", Roles: map[string]struct{}{"tenant:operator": {}}, Scopes: scopeSet}}
	mux := http.NewServeMux()
	handler := NewInvestigationHandler(repository, provider)
	mux.HandleFunc("GET /api/investigation/saved-views", handler.SavedViews)
	mux.HandleFunc("POST /api/investigation/saved-views", handler.CreateSavedView)
	mux.HandleFunc("PUT /api/investigation/saved-views/{savedViewId}", handler.RenameSavedView)
	mux.HandleFunc("DELETE /api/investigation/saved-views/{savedViewId}", handler.DeleteSavedView)
	return middleware.Correlation(mux)
}

func TestSavedViewsRequireDedicatedReadAndWriteScopes(t *testing.T) {
	readRepository := &savedViewRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	readResponse := httptest.NewRecorder()
	savedViewHandler(readRepository, "investigation:read", "accounts:read").ServeHTTP(readResponse, httptest.NewRequest(http.MethodGet, "/api/investigation/saved-views", nil))
	if readResponse.Code != http.StatusOK || !readRepository.called || !readRepository.access.Accounts {
		t.Fatalf("status=%d called=%v access=%#v body=%s", readResponse.Code, readRepository.called, readRepository.access, readResponse.Body.String())
	}

	writeRepository := &savedViewRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	request := httptest.NewRequest(http.MethodPost, "/api/investigation/saved-views", strings.NewReader(`{"name":"Active accounts","filter_schema_version":"1","domain":"accounts","filters":{"status":"active"}}`))
	request.Header.Set("Content-Type", "application/json")
	denied := httptest.NewRecorder()
	savedViewHandler(writeRepository, "investigation:read", "accounts:read").ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden || writeRepository.called {
		t.Fatalf("status=%d called=%v body=%s", denied.Code, writeRepository.called, denied.Body.String())
	}
}

func TestSavedViewCreateValidatesSchemaAndPassesAuditContext(t *testing.T) {
	repository := &savedViewRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	request := httptest.NewRequest(http.MethodPost, "/api/investigation/saved-views", strings.NewReader(`{"name":"Posted transfers","filter_schema_version":"1","domain":"transfers","filters":{"status":"posted","accountId":"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"}}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response := httptest.NewRecorder()
	savedViewHandler(repository, "investigation:read", "investigation:write", "transfers:read").ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !repository.called || repository.create.Filters["accountId"] != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" || !repository.create.Access.Transfers || repository.create.CorrelationID == "" || response.Header().Get("Location") == "" {
		t.Fatalf("status=%d called=%v command=%#v headers=%v body=%s", response.Code, repository.called, repository.create, response.Header(), response.Body.String())
	}

	for _, body := range []string{
		`{"name":"Unsafe","filter_schema_version":"1","domain":"accounts","filters":{"q":"free text"}}`,
		`{"name":"Cursor","filter_schema_version":"1","domain":"transfers","filters":{"cursor":"opaque"}}`,
		`{"name":"Extra","filter_schema_version":"1","domain":"accounts","filters":{"status":"active"},"result":{"balance":"1"}}`,
	} {
		repository := &savedViewRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
		request := httptest.NewRequest(http.MethodPost, "/api/investigation/saved-views", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		savedViewHandler(repository, "investigation:read", "investigation:write", "accounts:read", "transfers:read").ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || repository.called {
			t.Fatalf("accepted body=%s status=%d called=%v response=%s", body, response.Code, repository.called, response.Body.String())
		}
	}
}

func TestSavedViewRenameAndDeleteUseOptimisticVersions(t *testing.T) {
	viewID := "11111111-1111-4111-8111-111111111111"
	repository := &savedViewRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}}
	rename := httptest.NewRequest(http.MethodPut, "/api/investigation/saved-views/"+viewID, strings.NewReader(`{"expected_version":"3","name":"Renamed view"}`))
	rename.Header.Set("Content-Type", "application/json")
	renameResponse := httptest.NewRecorder()
	savedViewHandler(repository, "investigation:read", "investigation:write", "accounts:read").ServeHTTP(renameResponse, rename)
	if renameResponse.Code != http.StatusOK || repository.rename.ExpectedVersion != 3 || repository.rename.Name != "Renamed view" || !repository.rename.Access.Accounts {
		t.Fatalf("status=%d command=%#v body=%s", renameResponse.Code, repository.rename, renameResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/investigation/saved-views/"+viewID, nil)
	deleteRequest.Header.Set("If-Match", `"4"`)
	deleteResponse := httptest.NewRecorder()
	savedViewHandler(repository, "investigation:read", "investigation:write", "accounts:read").ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || repository.delete.ExpectedVersion != 4 || !repository.delete.Access.Accounts {
		t.Fatalf("status=%d command=%#v body=%s", deleteResponse.Code, repository.delete, deleteResponse.Body.String())
	}

	conflictRepository := &savedViewRepositoryStub{investigationRepositoryStub: &investigationRepositoryStub{}, mutationErr: investigation.ErrSavedViewVersion}
	conflict := httptest.NewRecorder()
	rename = httptest.NewRequest(http.MethodPut, "/api/investigation/saved-views/"+viewID, strings.NewReader(`{"expected_version":"3","name":"Renamed view"}`))
	rename.Header.Set("Content-Type", "application/json")
	savedViewHandler(conflictRepository, "investigation:read", "investigation:write", "accounts:read").ServeHTTP(conflict, rename)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "saved_view_version_conflict") || !errors.Is(conflictRepository.mutationErr, investigation.ErrSavedViewVersion) {
		t.Fatalf("status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}
