package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookverification"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

type verifiedWebhookEndpoint struct{}

func (verifiedWebhookEndpoint) Verify(context.Context, webhookverification.Job) (webhookverification.Outcome, error) {
	return webhookverification.Outcome{ResponseClass: "http_204"}, nil
}

func TestDeveloperWebhookLifecycleIsVerifiedVersionedAndAppendOnly(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository, err := db.NewDeveloperWebhookRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	replayRepository, err := db.NewDeliveryReplayRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service, err := developerplatform.NewWebhookService(repository, replayRepository, "production", func() time.Time { return now }, strings.NewReader(strings.Repeat("e", 64)))
	if err != nil {
		t.Fatal(err)
	}
	create := developerplatform.RegisterWebhookCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000711", IdempotencyKey: "webhook-register-0001", DisplayName: "Partner ledger", EndpointURL: "https://partner.example.test/hooks/ledgersync", SubscribedEvents: []string{"transfer.posted", "funding.posted"}, SigningKeyReference: "kms/webhook-001", SigningKeyID: "key-001"}
	created, err := service.Register(ctx, create)
	if err != nil || created.Webhook.Status != "pending_verification" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replayed, err := service.Register(ctx, create)
	if err != nil || !replayed.Replayed || replayed.Webhook.ID != created.Webhook.ID {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	verificationStore, err := db.NewWebhookVerificationJobRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	verificationWorker, err := webhookverification.NewWorker(verificationStore, verifiedWebhookEndpoint{}, func() time.Time { return now }, webhookverification.Config{WorkerID: "verification-test-worker"})
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := verificationWorker.RunOnce(ctx); err != nil || processed != 1 {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	verified, err := service.Get(ctx, testTenantID, created.Webhook.ID)
	if err != nil || verified.Status != "active" || verified.Version != "2" {
		t.Fatalf("verified=%#v err=%v", verified, err)
	}
	disabled, err := service.Disable(ctx, developerplatform.DisableWebhookCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000715", IdempotencyKey: "webhook-disable-0001", WebhookID: created.Webhook.ID, ExpectedVersion: 2, Reason: "Partner endpoint retired"})
	if err != nil || disabled.Webhook.Status != "disabled" || disabled.Webhook.Version != "3" {
		t.Fatalf("disabled=%#v err=%v", disabled, err)
	}
	if countRows(t, database, `SELECT count(*) FROM developer_webhook_events WHERE webhook_id=$1`, created.Webhook.ID) != 4 {
		t.Fatal("webhook event history was partial")
	}
	if _, err = database.ExecContext(ctx, `DELETE FROM developer_webhook_events WHERE webhook_id=$1`, created.Webhook.ID); err == nil {
		t.Fatal("append-only webhook history allowed deletion")
	}
	var digest []byte
	if err = database.QueryRowContext(ctx, `SELECT challenge_digest FROM developer_webhook_endpoints WHERE id=$1`, created.Webhook.ID).Scan(&digest); err != nil || digest != nil {
		t.Fatalf("challenge digest was retained after lifecycle completion: %x err=%v", digest, err)
	}
}
