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

type webhookHandlerRepository struct{}

func (*webhookHandlerRepository) RegisterWebhook(_ context.Context, c developerplatform.RegisterWebhookCommand, _ [sha256.Size]byte) (developerplatform.WebhookSubmission, error) {
	return developerplatform.WebhookSubmission{Webhook: developerplatform.Webhook{ID: "70000000-0000-4000-8000-000000000001", EndpointURL: c.EndpointURL, Status: "pending_verification", Version: "1"}}, nil
}
func (*webhookHandlerRepository) VerifyWebhook(context.Context, developerplatform.VerifyWebhookCommand, [sha256.Size]byte, [sha256.Size]byte) (developerplatform.WebhookSubmission, error) {
	return developerplatform.WebhookSubmission{}, nil
}
func (*webhookHandlerRepository) RotateWebhook(context.Context, developerplatform.RotateWebhookCommand, [sha256.Size]byte) (developerplatform.WebhookSubmission, error) {
	return developerplatform.WebhookSubmission{}, nil
}
func (*webhookHandlerRepository) DisableWebhook(context.Context, developerplatform.DisableWebhookCommand, [sha256.Size]byte) (developerplatform.WebhookSubmission, error) {
	return developerplatform.WebhookSubmission{}, nil
}
func (*webhookHandlerRepository) GetWebhook(context.Context, string, string) (developerplatform.Webhook, error) {
	return developerplatform.Webhook{}, developerplatform.ErrNotFound
}
func (*webhookHandlerRepository) ListWebhooks(context.Context, string, developerplatform.WebhookQuery) (developerplatform.WebhookPage, error) {
	return developerplatform.WebhookPage{}, nil
}
func (*webhookHandlerRepository) ListWebhookDeliveries(context.Context, string, string, developerplatform.DeliveryQuery) (developerplatform.DeliveryPage, error) {
	return developerplatform.DeliveryPage{}, nil
}

func TestDeveloperWebhookRegistrationIsStrictScopedAndOneTime(t *testing.T) {
	service, _ := developerplatform.NewWebhookService(&webhookHandlerRepository{}, nil, "production", func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }, strings.NewReader(strings.Repeat("d", 32)))
	handler := NewDeveloperWebhookHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant-1", Scopes: []string{"webhooks:write"}})
	router := http.NewServeMux()
	router.HandleFunc("POST /api/developer/webhooks", handler.Register)
	request := httptest.NewRequest(http.MethodPost, "/api/developer/webhooks", strings.NewReader(`{"display_name":"Accounting","endpoint_url":"https://partner.example.test/hooks/ledgersync","subscribed_events":["transfer.posted"],"signing_key_reference":"kms/webhook-001","signing_key_id":"key-001"}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "webhook-register-0001")
	response := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), `"verification_challenge"`) || strings.Contains(strings.ToLower(response.Body.String()), "signing_secret") || !strings.Contains(response.Body.String(), `"pending_verification"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	unsafe := httptest.NewRequest(http.MethodPost, "/api/developer/webhooks", strings.NewReader(`{"display_name":"Accounting","endpoint_url":"https://partner.example.test/hooks","subscribed_events":["transfer.posted"],"signing_key_reference":"kms/webhook-001","signing_key_id":"key-001","signing_secret":"forbidden"}`))
	unsafe.Header = request.Header.Clone()
	unsafeResponse := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(unsafeResponse, unsafe)
	if unsafeResponse.Code != http.StatusBadRequest {
		t.Fatalf("unsafe status=%d body=%s", unsafeResponse.Code, unsafeResponse.Body.String())
	}
}

func TestDeveloperWebhookRegistrationRequiresWriteScope(t *testing.T) {
	service, _ := developerplatform.NewWebhookService(&webhookHandlerRepository{}, nil, "production", time.Now, nil)
	handler := NewDeveloperWebhookHandler(service, identity.DevelopmentProvider{SubjectID: "reader", TenantID: "tenant-1", Scopes: []string{"webhooks:read"}})
	request := httptest.NewRequest(http.MethodPost, "/api/developer/webhooks", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer development-local-only")
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.Register)).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
