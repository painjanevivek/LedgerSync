package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

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
	if err != nil || created.Challenge == "" || created.Webhook.Status != "pending_verification" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replayed, err := service.Register(ctx, create)
	if err != nil || !replayed.Replayed || replayed.Challenge != "" || replayed.Webhook.ID != created.Webhook.ID {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	verified, err := service.Verify(ctx, developerplatform.VerifyWebhookCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000712", IdempotencyKey: "webhook-verify-00001", WebhookID: created.Webhook.ID, Challenge: created.Challenge, ExpectedVersion: 1})
	if err != nil || verified.Webhook.Status != "active" || verified.Webhook.Version != "2" {
		t.Fatalf("verified=%#v err=%v", verified, err)
	}
	if _, err = service.Rotate(ctx, developerplatform.RotateWebhookCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000713", IdempotencyKey: "webhook-rotate-00001", WebhookID: created.Webhook.ID, ExpectedVersion: 1, SigningKeyReference: "kms/webhook-002", SigningKeyID: "key-002"}); !errors.Is(err, developerplatform.ErrVersionConflict) {
		t.Fatalf("stale rotation error=%v", err)
	}
	rotated, err := service.Rotate(ctx, developerplatform.RotateWebhookCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000714", IdempotencyKey: "webhook-rotate-00002", WebhookID: created.Webhook.ID, ExpectedVersion: 2, SigningKeyReference: "kms/webhook-002", SigningKeyID: "key-002"})
	if err != nil || rotated.Webhook.Version != "3" || rotated.Webhook.SigningKeyID != "key-002" {
		t.Fatalf("rotated=%#v err=%v", rotated, err)
	}
	disabled, err := service.Disable(ctx, developerplatform.DisableWebhookCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000715", IdempotencyKey: "webhook-disable-0001", WebhookID: created.Webhook.ID, ExpectedVersion: 3, Reason: "Partner endpoint retired"})
	if err != nil || disabled.Webhook.Status != "disabled" || disabled.Webhook.Version != "4" {
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
