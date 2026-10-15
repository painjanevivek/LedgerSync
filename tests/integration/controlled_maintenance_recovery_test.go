package integration_test

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/retention"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"testing"
	"time"
)

func TestMaintenanceRetainsPublicationEvidenceAndDeniesAPI(t *testing.T) {
	transfers, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	if _, err := transfers.Submit(ctx, transferCommand(t, "retention-evidence-original", "1.00")); err != nil {
		t.Fatal(err)
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	queue, _ := db.NewOutboxRepository(worker.db, time.Now)
	events, err := queue.Claim(ctx, "synthetic-retention-worker", 10, time.Minute)
	if err != nil || len(events) != 3 {
		t.Fatalf("claim: %v, count=%d", err, len(events))
	}
	for _, event := range events {
		if err := queue.MarkPublished(ctx, "synthetic-retention-worker", event.ID, time.Now(), event.ClaimGeneration); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Add(31 * 24 * time.Hour)
	maintenance := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_maintenance")
	repo, _ := db.NewRetentionRepository(maintenance.db, func() time.Time { return now })
	policy := retention.Policy{TenantID: testTenantID, BatchSize: 100, PublishedOutboxAfter: 30 * 24 * time.Hour, RateWindowAfter: 2 * time.Hour}
	result, err := repo.Run(ctx, policy, true, "00000000-0000-4000-8000-000000000997")
	if err != nil || result.PublishedOutbox != 0 {
		t.Fatalf("retention with publication evidence: %v, %+v", err, result)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_events`); got != 3 {
		t.Fatal("retention deleted event evidence")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_publication_attempts`); got != 3 {
		t.Fatal("retention deleted publication evidence")
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	apiRepo, _ := db.NewRetentionRepository(api.db, func() time.Time { return now })
	if _, err := apiRepo.Run(ctx, policy, true, "00000000-0000-4000-8000-000000000998"); sqlState(err) != "42501" {
		t.Fatalf("API invoked maintenance: %v", err)
	}
}

func TestRecoveryApprovalExpiryAndTenantRoleAreEnforced(t *testing.T) {
	transfers, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	if _, err := transfers.Submit(ctx, transferCommand(t, "recovery-expiry-original", "1.00")); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,'expiry-approver','operator'),($1,'expiry-executor','operator')`, testTenantID); err != nil {
		t.Fatal(err)
	}
	var eventID string
	if err := admin.QueryRow(`UPDATE outbox_events SET dead_at=now() WHERE tenant_id=$1 RETURNING id`, testTenantID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repo, _ := db.NewOutboxReplayRepository(api.db, func() time.Time { return now })
	approval := recovery.Approval{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "missing-role", ReasonCode: "dependency_restored", CorrelationID: "00000000-0000-4000-8000-000000000999"}
	if err := repo.Approve(ctx, approval); !errors.Is(err, db.ErrReplayNotApproved) {
		t.Fatalf("unassigned actor approved replay: %v", err)
	}
	approval.ActorSubjectID = "expiry-approver"
	if err := repo.Approve(ctx, approval); err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	if err := repo.Replay(ctx, recovery.Replay{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "expiry-executor", CorrelationID: approval.CorrelationID}); !errors.Is(err, db.ErrReplayNotApproved) {
		t.Fatalf("expired approval replayed: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_replay_actions WHERE action='executed'`); got != 0 {
		t.Fatal("expired approval left an execution")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE id=$1 AND dead_at IS NOT NULL`, eventID); got != 1 {
		t.Fatal("expired approval changed dead event")
	}
}
