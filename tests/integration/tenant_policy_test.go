package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantPermanentPolicyCoverage(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	rows, err := admin.Query(`SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind='r' AND
 (EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attname='tenant_id' AND NOT a.attisdropped)
 OR c.relname IN ('tenants','account_balance_projections','account_opening_balances','outbox_publication_attempts'))
 AND (NOT c.relrowsecurity OR NOT c.relforcerowsecurity)
 ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		t.Errorf("tenant relation lacks forced row security: %s", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantWorkerClaimBoundary(t *testing.T) {
	transfers, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const endpoint = "00000000-0000-4000-8000-000000000971"
	if _, err := admin.ExecContext(ctx, `INSERT INTO developer_webhook_endpoints(id,tenant_id,display_name,endpoint_url,subscribed_events,signing_key_reference,signing_key_id,status,version,verified_at,created_at,updated_at)
 VALUES($1,$2,'Synthetic receiver','https://partner.example.test/hooks',ARRAY['transfer.posted'],'kms/synthetic','synthetic','active',1,now(),now(),now())`, endpoint, testTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := transfers.Submit(ctx, transferCommand(t, "tenant-worker-boundary-001", "1.00")); err != nil {
		t.Fatal(err)
	}
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	if _, err := admin.Exec(`REVOKE UPDATE ON outbox_events,outbox_publication_attempts,webhook_delivery_jobs,webhook_endpoint_verification_jobs FROM ledgersync_worker; REVOKE INSERT ON delivery_attempts,outbox_publication_attempts,logical_message_receipts FROM ledgersync_worker`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provisionLiveMigrationRoles(t, admin) })
	session.db.SetMaxOpenConns(1)
	var count int
	if err := session.db.QueryRowContext(ctx, `SELECT count(*) FROM outbox_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unscoped worker read count=%d err=%v", count, err)
	}
	queue, _ := db.NewOutboxRepository(session.db, time.Now)
	events, err := queue.Claim(ctx, "scoped-worker", 10, time.Minute)
	if err != nil || len(events) == 0 {
		t.Fatalf("bounded claim returned %d events: %v", len(events), err)
	}
	for _, event := range events {
		var hidden sql.NullString
		if err := session.db.QueryRowContext(ctx, `SELECT controlled_claim_tenant_v1('outbox',$1,'wrong-owner',$2)`, event.ID, event.ClaimGeneration).Scan(&hidden); err != nil || hidden.Valid {
			t.Fatalf("foreign claim resolved: %v", err)
		}
		if err := queue.RenewClaim(ctx, "scoped-worker", event.ID, event.ClaimGeneration, time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := queue.MarkPublished(ctx, "scoped-worker", event.ID, time.Now(), event.ClaimGeneration); err != nil {
			t.Fatal(err)
		}
		if err := queue.MarkPublished(ctx, "scoped-worker", event.ID, time.Now(), event.ClaimGeneration); err == nil {
			t.Fatal("stale completion accepted")
		}
	}
	deliveries, _ := db.NewWebhookDeliveryJobRepository(session.db, time.Now)
	jobs, err := deliveries.Claim(ctx, "scoped-worker", 10, time.Minute)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("scoped delivery claim count=%d err=%v", len(jobs), err)
	}
	job := jobs[0]
	if err := deliveries.RenewClaim(ctx, "scoped-worker", job.ID, job.ClaimGeneration, time.Minute); err != nil {
		t.Fatal(err)
	}
	completion := webhookdelivery.Completion{JobID: job.ID, WorkerID: "scoped-worker", AttemptNumber: job.AttemptNumber, ClaimGeneration: job.ClaimGeneration, Status: webhookdelivery.StatusDelivered, CompletedAt: time.Now(), ResponseClass: "http_204"}
	wrongAttempt := completion
	wrongAttempt.AttemptNumber++
	if err := deliveries.Complete(ctx, wrongAttempt); err == nil {
		t.Fatal("worker completed a different delivery attempt")
	}
	if err := deliveries.Complete(ctx, completion); err != nil {
		t.Fatal(err)
	}
	if err := deliveries.Complete(ctx, completion); err == nil {
		t.Fatal("stale webhook completion accepted")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM delivery_attempts WHERE tenant_id=$1 AND endpoint_reference=$2`, testTenantID, endpoint); got != 1 {
		t.Fatalf("delivery evidence count=%d", got)
	}
	if err := session.db.QueryRowContext(ctx, `SELECT count(*) FROM outbox_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("worker scope leaked count=%d err=%v", count, err)
	}
	var inherited bool
	if err := session.db.QueryRowContext(ctx, `SELECT pg_has_role(current_user,'ledgersync_queue_owner','MEMBER')`).Scan(&inherited); err != nil || inherited {
		t.Fatalf("worker can inherit function owner: %v", err)
	}
}
