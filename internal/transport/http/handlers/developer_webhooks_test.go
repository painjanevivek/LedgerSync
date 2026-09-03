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
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type webhookHandlerRepository struct{}

type webhookReplayHandlerRepository struct {
	approval recovery.DeliveryApproval
	replay   recovery.DeliveryReplay
}

func (r *webhookReplayHandlerRepository) Inspect(context.Context, string, string) (recovery.DeadDelivery, error) {
	return recovery.DeadDelivery{AttemptID: "70000000-0000-4000-8000-000000000002", Kind: "webhook", EndpointReference: "70000000-0000-4000-8000-000000000001"}, nil
}
func (r *webhookReplayHandlerRepository) Approve(_ context.Context, command recovery.DeliveryApproval) (recovery.DeliveryApprovalResult, error) {
	r.approval = command
	return recovery.DeliveryApprovalResult{ApprovalID: "70000000-0000-4000-8000-000000000003", Replayed: true}, nil
}
func (r *webhookReplayHandlerRepository) Replay(_ context.Context, command recovery.DeliveryReplay) (recovery.DeliveryReplayResult, error) {
	r.replay = command
	return recovery.DeliveryReplayResult{DeliveryJobID: "70000000-0000-4000-8000-000000000004", Replayed: true}, nil
}

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

func TestDeveloperWebhookReplayCarriesStableIdentityAndApprovalReference(t *testing.T) {
	replayRepository := &webhookReplayHandlerRepository{}
	service, _ := developerplatform.NewWebhookService(&webhookHandlerRepository{}, replayRepository, "production", time.Now, nil)
	handler := NewDeveloperWebhookHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant-1", Scopes: []string{"webhooks:replay"}})
	router := http.NewServeMux()
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/deliveries/{attemptId}/replay-approvals", handler.ApproveReplay)
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/deliveries/{attemptId}/replays", handler.Replay)

	approvalRequest := httptest.NewRequest(http.MethodPost, "/api/developer/webhooks/70000000-0000-4000-8000-000000000001/deliveries/70000000-0000-4000-8000-000000000002/replay-approvals", strings.NewReader(`{"reason_code":"endpoint_restored"}`))
	approvalRequest.Header.Set("Authorization", "Bearer development-local-only")
	approvalRequest.Header.Set("Content-Type", "application/json")
	approvalRequest.Header.Set("Idempotency-Key", "delivery-approval-0001")
	approvalResponse := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(approvalResponse, approvalRequest)
	if approvalResponse.Code != http.StatusCreated || approvalResponse.Header().Get("Idempotent-Replay") != "true" || !strings.Contains(approvalResponse.Body.String(), `"approval_id":"70000000-0000-4000-8000-000000000003"`) || replayRepository.approval.IdempotencyKey != "delivery-approval-0001" {
		t.Fatalf("approval status=%d header=%q body=%s command=%+v", approvalResponse.Code, approvalResponse.Header().Get("Idempotent-Replay"), approvalResponse.Body.String(), replayRepository.approval)
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/developer/webhooks/70000000-0000-4000-8000-000000000001/deliveries/70000000-0000-4000-8000-000000000002/replays", strings.NewReader(`{"approval_id":"70000000-0000-4000-8000-000000000003"}`))
	replayRequest.Header = approvalRequest.Header.Clone()
	replayRequest.Header.Set("Idempotency-Key", "delivery-execution-0001")
	replayResponse := httptest.NewRecorder()
	middleware.Correlation(router).ServeHTTP(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusAccepted || replayResponse.Header().Get("Idempotent-Replay") != "true" || !strings.Contains(replayResponse.Body.String(), `"delivery_job_id":"70000000-0000-4000-8000-000000000004"`) || replayRepository.replay.ApprovalID != "70000000-0000-4000-8000-000000000003" || replayRepository.replay.IdempotencyKey != "delivery-execution-0001" {
		t.Fatalf("replay status=%d header=%q body=%s command=%+v", replayResponse.Code, replayResponse.Header().Get("Idempotent-Replay"), replayResponse.Body.String(), replayRepository.replay)
	}
}
