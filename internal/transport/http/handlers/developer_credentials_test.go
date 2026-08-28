package handlers

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type developerCredentialHandlerRepository struct {
	command developerplatform.CreateCredentialCommand
}

func (r *developerCredentialHandlerRepository) CreateCredential(_ context.Context, command developerplatform.CreateCredentialCommand, _ [sha256.Size]byte) (developerplatform.CredentialSubmission, error) {
	r.command = command
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	return developerplatform.CredentialSubmission{Credential: developerplatform.Credential{ID: "70000000-0000-4000-8000-000000000001", DisplayName: command.DisplayName, ExternalReference: command.ExternalReference, Audience: command.Audience, Scopes: command.Scopes, Status: "active", Version: "1", ExpiresAt: command.ExpiresAt, CreatedAt: now, UpdatedAt: now}}, nil
}
func (*developerCredentialHandlerRepository) RotateCredential(context.Context, developerplatform.RotateCredentialCommand, [sha256.Size]byte) (developerplatform.CredentialSubmission, error) {
	return developerplatform.CredentialSubmission{}, nil
}
func (*developerCredentialHandlerRepository) RevokeCredential(context.Context, developerplatform.RevokeCredentialCommand, [sha256.Size]byte) (developerplatform.CredentialSubmission, error) {
	return developerplatform.CredentialSubmission{}, nil
}
func (*developerCredentialHandlerRepository) GetCredential(context.Context, string, string) (developerplatform.Credential, error) {
	return developerplatform.Credential{}, developerplatform.ErrNotFound
}
func (*developerCredentialHandlerRepository) ListCredentials(context.Context, string, developerplatform.CredentialQuery) (developerplatform.CredentialPage, error) {
	return developerplatform.CredentialPage{Items: []developerplatform.Credential{}}, nil
}

func TestDeveloperCredentialCreateIsScopedStrictAndNonSecret(t *testing.T) {
	repository := &developerCredentialHandlerRepository{}
	service, _ := developerplatform.NewCredentialService(repository, func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) })
	handler := NewDeveloperCredentialHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant-1", Scopes: []string{"credentials:write"}})
	router := http.NewServeMux()
	router.HandleFunc("POST /api/developer/credentials", handler.Create)

	request := httptest.NewRequest(http.MethodPost, "/api/developer/credentials", strings.NewReader(`{"display_name":"Partner","external_reference":"cognito/client-001","audience":"ledgersync-api","scopes":["accounts:read"],"expires_at":"2026-08-29T12:00:00Z"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "credential-key-0001")
	response := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"external_reference":"cognito/client-001"`) || strings.Contains(strings.ToLower(response.Body.String()), "secret") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repository.command.CorrelationID == "" || repository.command.ActorSubjectID != "operator" {
		t.Fatalf("command=%#v", repository.command)
	}

	unsafe := httptest.NewRequest(http.MethodPost, "/api/developer/credentials", strings.NewReader(`{"display_name":"Partner","external_reference":"cognito/client-001","audience":"ledgersync-api","scopes":["accounts:read"],"expires_at":"2026-08-29T12:00:00Z","client_secret":"forbidden"}`))
	unsafe.Header = request.Header.Clone()
	unsafeResponse := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(unsafeResponse, unsafe)
	if unsafeResponse.Code != http.StatusBadRequest {
		t.Fatalf("unsafe status=%d body=%s", unsafeResponse.Code, unsafeResponse.Body.String())
	}
}

func TestDeveloperCredentialCreateRequiresWriteScope(t *testing.T) {
	repository := &developerCredentialHandlerRepository{}
	service, _ := developerplatform.NewCredentialService(repository, time.Now)
	handler := NewDeveloperCredentialHandler(service, identity.DevelopmentProvider{SubjectID: "reader", TenantID: "tenant-1", Scopes: []string{"credentials:read"}})
	request := httptest.NewRequest(http.MethodPost, "/api/developer/credentials", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.Create)).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
