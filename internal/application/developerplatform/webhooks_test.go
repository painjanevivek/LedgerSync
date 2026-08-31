package developerplatform

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

type webhookRepositoryStub struct{ command RegisterWebhookCommand }

func (r *webhookRepositoryStub) RegisterWebhook(_ context.Context, c RegisterWebhookCommand, _ [sha256.Size]byte) (WebhookSubmission, error) {
	r.command = c
	return WebhookSubmission{Webhook: Webhook{ID: "70000000-0000-4000-8000-000000000001", EndpointURL: c.EndpointURL, Status: "pending_verification", Version: "1"}}, nil
}
func (*webhookRepositoryStub) VerifyWebhook(context.Context, VerifyWebhookCommand, [sha256.Size]byte, [sha256.Size]byte) (WebhookSubmission, error) {
	return WebhookSubmission{}, nil
}
func (*webhookRepositoryStub) RotateWebhook(context.Context, RotateWebhookCommand, [sha256.Size]byte) (WebhookSubmission, error) {
	return WebhookSubmission{}, nil
}
func (*webhookRepositoryStub) DisableWebhook(context.Context, DisableWebhookCommand, [sha256.Size]byte) (WebhookSubmission, error) {
	return WebhookSubmission{}, nil
}
func (*webhookRepositoryStub) GetWebhook(context.Context, string, string) (Webhook, error) {
	return Webhook{}, ErrNotFound
}
func (*webhookRepositoryStub) ListWebhooks(context.Context, string, WebhookQuery) (WebhookPage, error) {
	return WebhookPage{}, nil
}
func (*webhookRepositoryStub) ListWebhookDeliveries(context.Context, string, string, DeliveryQuery) (DeliveryPage, error) {
	return DeliveryPage{}, nil
}

func TestWebhookRegistrationEnforcesEnvironmentAndStoresDigestOnly(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository := &webhookRepositoryStub{}
	service, _ := NewWebhookService(repository, nil, "production", func() time.Time { return now }, strings.NewReader(strings.Repeat("a", 32)))
	base := RegisterWebhookCommand{TenantID: "tenant", ActorSubjectID: "actor", CorrelationID: "correlation", IdempotencyKey: "webhook-register-0001", DisplayName: "Accounting", EndpointURL: "https://partner.example.test/ledgersync", SubscribedEvents: []string{"transfer.posted"}, SigningKeyReference: "kms/webhook-001", SigningKeyID: "key-001"}
	submission, err := service.Register(context.Background(), base)
	if err != nil || submission.Webhook.Status != "pending_verification" {
		t.Fatalf("submission=%#v err=%v", submission, err)
	}
	if repository.command.VerificationChallenge == "" || repository.command.ChallengeDigest == ([sha256.Size]byte{}) || repository.command.ChallengeExpiresAt != now.Add(10*time.Minute) {
		t.Fatalf("command=%#v", repository.command)
	}
	unsafe := base
	unsafe.IdempotencyKey = "webhook-register-0002"
	unsafe.EndpointURL = "http://169.254.169.254/latest/meta-data"
	if _, err = service.Register(context.Background(), unsafe); err != ErrInvalidCommand {
		t.Fatalf("unsafe url error=%v", err)
	}
	unsafe.EndpointURL = "https://127.0.0.1/hooks"
	if _, err = service.Register(context.Background(), unsafe); err != ErrInvalidCommand {
		t.Fatalf("production loopback error=%v", err)
	}
	local, _ := NewWebhookService(repository, nil, "sandbox", func() time.Time { return now }, strings.NewReader(strings.Repeat("b", 32)))
	unsafe.EndpointURL = "http://127.0.0.1:3000/hooks"
	if _, err = local.Register(context.Background(), unsafe); err != nil {
		t.Fatalf("loopback url error=%v", err)
	}
}

func TestWebhookRegistrationRejectsQueryCredentialsAndUnknownEvents(t *testing.T) {
	repository := &webhookRepositoryStub{}
	service, _ := NewWebhookService(repository, nil, "production", time.Now, strings.NewReader(strings.Repeat("c", 32)))
	command := RegisterWebhookCommand{TenantID: "tenant", ActorSubjectID: "actor", CorrelationID: "correlation", IdempotencyKey: "webhook-register-0003", DisplayName: "Unsafe", EndpointURL: "https://partner.example.test/hook?token=secret", SubscribedEvents: []string{"transfer.posted"}, SigningKeyReference: "kms/webhook-001", SigningKeyID: "key-001"}
	if _, err := service.Register(context.Background(), command); err != ErrInvalidCommand {
		t.Fatalf("query credential error=%v", err)
	}
	command.EndpointURL = "https://partner.example.test/hook"
	command.SubscribedEvents = []string{"arbitrary.event"}
	if _, err := service.Register(context.Background(), command); err != ErrInvalidCommand {
		t.Fatalf("unknown event error=%v", err)
	}
}
