package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantDeveloperReadsCarryAuthenticatedActor(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC()
	credentials, _ := db.NewDeveloperCredentialRepository(admin, func() time.Time { return now })
	credentialService, _ := developerplatform.NewCredentialService(credentials, func() time.Time { return now })
	created, err := credentialService.Create(ctx, developerplatform.CreateCredentialCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000701", IdempotencyKey: "tenant-credential-context-001",
		DisplayName: "Synthetic integration", ExternalReference: "synthetic-context-client", Audience: "ledgersync-api", Scopes: []string{"accounts:read"}, ExpiresAt: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	webhooks, _ := db.NewDeveloperWebhookRepository(admin, func() time.Time { return now })
	replays, _ := db.NewDeliveryReplayRepository(admin, func() time.Time { return now })
	webhookService, err := developerplatform.NewWebhookService(webhooks, replays, "production", func() time.Time { return now }, strings.NewReader(strings.Repeat("e", 64)))
	if err != nil {
		t.Fatal(err)
	}
	webhook, err := webhookService.Register(ctx, developerplatform.RegisterWebhookCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000702", IdempotencyKey: "tenant-webhook-context-001",
		DisplayName: "Synthetic endpoint", EndpointURL: "https://partner.example.test/hooks", SubscribedEvents: []string{"transfer.posted"}, SigningKeyReference: "kms/synthetic-context", SigningKeyID: "synthetic-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"developer_credentials", "developer_webhook_endpoints", "delivery_attempts"} {
		// Requiring the exact actor proves propagation, not just a nonempty value.
		if _, err := admin.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ENABLE ROW LEVEL SECURITY;
CREATE POLICY portfolio_developer_scope ON %s AS RESTRICTIVE USING (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid AND current_setting('app.actor_id',true)='%s')`, table, table, testActorID)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.ExecContext(context.Background(), fmt.Sprintf(`DROP POLICY portfolio_developer_scope ON %s`, table)); err != nil {
				t.Error(err)
			}
		})
	}
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	session.db.SetMaxOpenConns(1)
	credentials, _ = db.NewDeveloperCredentialRepository(session.db, func() time.Time { return now })
	webhooks, _ = db.NewDeveloperWebhookRepository(session.db, func() time.Time { return now })
	t.Run("credential detail and list", func(t *testing.T) {
		item, err := credentials.GetCredential(ctx, testTenantID, testActorID, created.Credential.ID)
		if err != nil || item.ID != created.Credential.ID {
			t.Fatalf("credential detail: %v", err)
		}
		page, err := credentials.ListCredentials(ctx, testTenantID, testActorID, developerplatform.CredentialQuery{Limit: 10})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("credential list: %v", err)
		}
		if _, err := credentials.GetCredential(ctx, "00000000-0000-4000-8000-000000000902", testActorID, created.Credential.ID); !errors.Is(err, developerplatform.ErrNotFound) {
			t.Fatalf("foreign tenant detail: %v", err)
		}
	})
	t.Run("webhook detail list and delivery evidence", func(t *testing.T) {
		item, err := webhooks.GetWebhook(ctx, testTenantID, testActorID, webhook.Webhook.ID)
		if err != nil || item.ID != webhook.Webhook.ID {
			t.Fatalf("webhook detail: %v", err)
		}
		page, err := webhooks.ListWebhooks(ctx, testTenantID, testActorID, developerplatform.WebhookQuery{Limit: 10})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("webhook list: %v", err)
		}
		if _, err := webhooks.ListWebhookDeliveries(ctx, testTenantID, testActorID, webhook.Webhook.ID, developerplatform.DeliveryQuery{Limit: 10}); err != nil {
			t.Fatal(err)
		}
	})
}
