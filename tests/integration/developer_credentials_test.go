package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestDeveloperCredentialLifecycleIsIdempotentVersionedAndNonSecret(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository, err := db.NewDeveloperCredentialRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service, err := developerplatform.NewCredentialService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	create := developerplatform.CreateCredentialCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000701", IdempotencyKey: "credential-create-0001",
		DisplayName: "Partner API", ExternalReference: "partner-client-001", Audience: "ledgersync-api", Scopes: []string{"accounts:read", "transfers:write"}, ExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	created, err := service.Create(ctx, create)
	if err != nil || created.Replayed || created.Credential.Status != "active" || created.Credential.Version != "1" || created.Credential.LastUsedAt != nil {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replayed, err := service.Create(ctx, create)
	if err != nil || !replayed.Replayed || replayed.Credential.ID != created.Credential.ID {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	changed := create
	changed.DisplayName = "Changed intent"
	if _, err = service.Create(ctx, changed); !errors.Is(err, developerplatform.ErrIdempotencyConflict) {
		t.Fatalf("changed-intent error=%v", err)
	}
	if err = repository.RecordUsage(ctx, testTenantID, "partner-client-001", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	used, err := service.Get(ctx, testTenantID, created.Credential.ID)
	if err != nil || used.LastUsedAt == nil || !used.LastUsedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("used=%#v err=%v", used, err)
	}
	rotated, err := service.Rotate(ctx, developerplatform.RotateCredentialCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000702", IdempotencyKey: "credential-rotate-0001",
		CredentialID: created.Credential.ID, ExpectedVersion: 1, ExternalReference: "partner-client-002", Audience: "ledgersync-api", Scopes: []string{"accounts:read", "transfers:read"}, ExpiresAt: now.Add(180 * 24 * time.Hour),
	})
	if err != nil || rotated.Credential.Version != "2" || rotated.Credential.ExternalReference != "partner-client-002" {
		t.Fatalf("rotated=%#v err=%v", rotated, err)
	}
	if _, err = service.Rotate(ctx, developerplatform.RotateCredentialCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000703", IdempotencyKey: "credential-rotate-0002", CredentialID: created.Credential.ID, ExpectedVersion: 1, ExternalReference: "partner-client-003", Audience: "ledgersync-api", Scopes: []string{"accounts:read"}, ExpiresAt: now.Add(90 * 24 * time.Hour)}); !errors.Is(err, developerplatform.ErrVersionConflict) {
		t.Fatalf("stale rotation error=%v", err)
	}
	revoked, err := service.Revoke(ctx, developerplatform.RevokeCredentialCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-4000-8000-000000000704", IdempotencyKey: "credential-revoke-0001", CredentialID: created.Credential.ID, ExpectedVersion: 2, Reason: "Partner integration retired"})
	if err != nil || revoked.Credential.Status != "revoked" || revoked.Credential.Version != "3" || revoked.Credential.RevokedAt == nil {
		t.Fatalf("revoked=%#v err=%v", revoked, err)
	}
	page, err := service.List(ctx, testTenantID, developerplatform.CredentialQuery{Status: "revoked", Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != created.Credential.ID {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if countRows(t, database, `SELECT count(*) FROM developer_credential_events WHERE credential_id=$1`, created.Credential.ID) != 3 || countRows(t, database, `SELECT count(*) FROM developer_command_idempotency WHERE tenant_id=$1 AND state='completed'`, testTenantID) != 3 {
		t.Fatal("credential history or retry evidence was partial")
	}
	if _, err = database.ExecContext(ctx, `DELETE FROM developer_credential_events WHERE credential_id=$1`, created.Credential.ID); err == nil {
		t.Fatal("append-only credential history allowed deletion")
	}
}
