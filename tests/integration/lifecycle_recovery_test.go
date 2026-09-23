package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/provisioning"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/retention"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestRetentionIsBoundedAndProtectsFinalEvidence(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	if _, err := service.Submit(ctx, transferCommand(t, "retention-key-00000000001", "100")); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE outbox_events SET published_at='2026-08-18T10:00:00Z' WHERE tenant_id=$1`, testTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO api_rate_limit_windows(tenant_id,principal_hash,route_key,window_started_at,request_count)VALUES($1,decode('0011','hex'),'POST /transfers','2026-08-18T10:00:00Z',1)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewRetentionRepository(database, func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	policy := retention.Policy{TenantID: testTenantID, BatchSize: 1, PublishedOutboxAfter: 30 * 24 * time.Hour, RateWindowAfter: 2 * time.Hour}
	dryRun, err := repository.Run(ctx, policy, false, "00000000-0000-0000-0000-000000000201")
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.PublishedOutbox != 3 || dryRun.RetainedIdempotency != 1 || dryRun.ExpiredRates != 1 {
		t.Fatalf("unexpected dry-run counts: %+v", dryRun)
	}
	if countRows(t, database, `SELECT count(*) FROM outbox_events`) != 3 {
		t.Fatal("dry-run changed outbox state")
	}
	applied, err := repository.Run(ctx, policy, true, "00000000-0000-0000-0000-000000000202")
	if err != nil {
		t.Fatal(err)
	}
	if applied.PublishedOutbox != 1 || applied.RetainedIdempotency != 1 || applied.ExpiredRates != 1 {
		t.Fatalf("unexpected apply counts: %+v", applied)
	}
	if countRows(t, database, `SELECT count(*) FROM outbox_events`) != 2 || countRows(t, database, `SELECT count(*) FROM idempotency_requests`) != 1 || countRows(t, database, `SELECT count(*) FROM api_rate_limit_windows`) != 0 {
		t.Fatal("retention did not delete only eligible disposable rows")
	}
	if countRows(t, database, `SELECT count(*) FROM retention_runs`) != 2 || countRows(t, database, `SELECT count(*) FROM audit_events WHERE event_type='retention.completed'`) != 2 {
		t.Fatal("retention evidence was not persisted")
	}
}

func TestLifecycleMigrationProvidesStableHistoryAndRetentionIndexes(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	for _, indexName := range []string{
		"transfers_account_history_stable_idx",
		"transfers_credit_history_stable_idx",
		"outbox_published_retention_idx",
		"idempotency_expiry_retention_idx",
	} {
		var exists bool
		if err := database.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, indexName).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("required lifecycle index %q is missing", indexName)
		}
	}
}

func TestDeadOutboxAndDeliveryReplayRequireApprovalAndSeparation(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	result, err := service.Submit(ctx, transferCommand(t, "recovery-key-00000000001", "100"))
	if err != nil {
		t.Fatal(err)
	}
	var eventID string
	if err = database.QueryRowContext(ctx, `UPDATE outbox_events SET dead_at='2026-08-18T10:00:00Z',last_error_code='redis_unavailable' WHERE tenant_id=$1 RETURNING id`, testTenantID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	outbox, err := db.NewOutboxReplayRepository(database, func() time.Time { return time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = outbox.Inspect(ctx, testTenantID, eventID); err != nil {
		t.Fatal(err)
	}
	correlation := "00000000-0000-0000-0000-000000000211"
	if err = outbox.Approve(ctx, recovery.Approval{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "approver", ReasonCode: "dependency_restored", CorrelationID: correlation}); err != nil {
		t.Fatal(err)
	}
	if err = outbox.Approve(ctx, recovery.Approval{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "approver", ReasonCode: "dependency_restored", CorrelationID: correlation}); err != nil {
		t.Fatalf("identical approval retry must be idempotent: %v", err)
	}
	if err = outbox.Replay(ctx, recovery.Replay{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "approver", CorrelationID: correlation}); !errors.Is(err, db.ErrReplaySeparationRequired) {
		t.Fatalf("same-operator replay error=%v", err)
	}
	if err = outbox.Replay(ctx, recovery.Replay{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "executor", CorrelationID: correlation}); err != nil {
		t.Fatal(err)
	}
	if err = outbox.Replay(ctx, recovery.Replay{TenantID: testTenantID, EventID: eventID, ActorSubjectID: "executor", CorrelationID: correlation}); !errors.Is(err, db.ErrDeadOutboxNotFound) {
		t.Fatalf("duplicate replay error=%v", err)
	}
	if countRows(t, database, `SELECT count(*) FROM outbox_replay_actions WHERE action='executed'`) != 1 {
		t.Fatal("outbox replay executed more than once")
	}

	webhookID := "00000000-0000-0000-0000-000000000223"
	if _, err = database.ExecContext(ctx, `INSERT INTO developer_webhook_endpoints(id,tenant_id,display_name,endpoint_url,subscribed_events,signing_key_reference,signing_key_id,status,version,verified_at,created_at,updated_at)VALUES($1,$2,'Recovery partner','https://partner.example.test/hooks',ARRAY['transfer.posted'],'secrets/webhooks/recovery','recovery-key','active',1,'2026-08-18T09:00:00Z','2026-08-18T09:00:00Z','2026-08-18T09:00:00Z')`, webhookID, testTenantID); err != nil {
		t.Fatal(err)
	}
	var transferWebhookEventID string
	if err = database.QueryRowContext(ctx, `SELECT id::text FROM outbox_events WHERE tenant_id=$1 AND transfer_id=$2 AND event_type='transfer.posted.v1'`, testTenantID, result.Result.TransferID).Scan(&transferWebhookEventID); err != nil {
		t.Fatal(err)
	}
	deadAttemptID := "00000000-0000-0000-0000-000000000221"
	if _, err = database.ExecContext(ctx, `INSERT INTO delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,sanitized_error_code,due_at,completed_at)VALUES($1,$2,$3,$4,'webhook',$5,1,'dead','timeout','2026-08-18T10:00:00Z','2026-08-18T10:01:00Z')`, deadAttemptID, testTenantID, result.Result.TransferID, transferWebhookEventID, webhookID); err != nil {
		t.Fatal(err)
	}
	delivery, err := db.NewDeliveryReplayRepository(database, func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	deliveryCorrelation := "00000000-0000-0000-0000-000000000222"
	approvalKey := "delivery-approval-0001"
	approved, err := delivery.Approve(ctx, recovery.DeliveryApproval{TenantID: testTenantID, AttemptID: deadAttemptID, ActorSubjectID: "approver", ReasonCode: "endpoint_restored", CorrelationID: deliveryCorrelation, IdempotencyKey: approvalKey})
	if err != nil || approved.ApprovalID == "" || approved.Replayed {
		t.Fatal(err)
	}
	approvedRetry, err := delivery.Approve(ctx, recovery.DeliveryApproval{TenantID: testTenantID, AttemptID: deadAttemptID, ActorSubjectID: "approver", ReasonCode: "endpoint_restored", CorrelationID: "00000000-0000-0000-0000-000000000224", IdempotencyKey: approvalKey})
	if err != nil || approvedRetry.ApprovalID != approved.ApprovalID || !approvedRetry.Replayed {
		t.Fatalf("identical delivery approval retry=%+v error=%v", approvedRetry, err)
	}
	executionKey := "delivery-execution-0001"
	if _, err = delivery.Replay(ctx, recovery.DeliveryReplay{TenantID: testTenantID, AttemptID: deadAttemptID, ApprovalID: approved.ApprovalID, ActorSubjectID: "approver", CorrelationID: deliveryCorrelation, IdempotencyKey: executionKey}); !errors.Is(err, db.ErrReplaySeparationRequired) {
		t.Fatalf("same-operator delivery replay error=%v", err)
	}
	replayed, err := delivery.Replay(ctx, recovery.DeliveryReplay{TenantID: testTenantID, AttemptID: deadAttemptID, ApprovalID: approved.ApprovalID, ActorSubjectID: "executor", CorrelationID: deliveryCorrelation, IdempotencyKey: executionKey})
	if err != nil || replayed.DeliveryJobID == "" || replayed.Replayed {
		t.Fatalf("delivery replay=%+v error=%v", replayed, err)
	}
	replayedRetry, err := delivery.Replay(ctx, recovery.DeliveryReplay{TenantID: testTenantID, AttemptID: deadAttemptID, ApprovalID: approved.ApprovalID, ActorSubjectID: "executor", CorrelationID: "00000000-0000-0000-0000-000000000225", IdempotencyKey: executionKey})
	if err != nil || replayedRetry.DeliveryJobID != replayed.DeliveryJobID || !replayedRetry.Replayed {
		t.Fatalf("delivery replay retry=%+v error=%v", replayedRetry, err)
	}
	if _, err = delivery.Replay(ctx, recovery.DeliveryReplay{TenantID: testTenantID, AttemptID: deadAttemptID, ApprovalID: approved.ApprovalID, ActorSubjectID: "executor", CorrelationID: deliveryCorrelation, IdempotencyKey: "delivery-execution-0002"}); !errors.Is(err, db.ErrDeliveryReplayIdempotencyConflict) {
		t.Fatalf("changed delivery replay key error=%v", err)
	}
	if countRows(t, database, `SELECT count(*) FROM delivery_attempts WHERE transfer_id=$1`, result.Result.TransferID) != 1 || countRows(t, database, `SELECT count(*) FROM webhook_delivery_jobs WHERE replay_of_attempt_id=$1`, deadAttemptID) != 1 || countRows(t, database, `SELECT count(*) FROM delivery_replay_actions WHERE action='executed'`) != 1 {
		t.Fatal("delivery replay was not exactly once")
	}
}

func TestPartnerProvisioningIsAuditedIdempotentAndRollbackSafe(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	configuration := provisioning.Config{
		TenantID: "00000000-0000-0000-0000-000000000301", ExternalReference: "design-partner-301", Currency: "USD",
		MinimumTransferMinor: "1", MaximumTransferMinor: "100000", ActorRolling24hMinor: "500000", SourceRolling24hMinor: "500000", TenantRolling24hMinor: "1000000",
		Subjects:    []provisioning.Subject{{ID: "partner-operator", Roles: []string{"operator", "finance"}}},
		Credentials: []provisioning.Credential{{Reference: "idp://pilot/client-301", Audience: "ledgersync-api", Scopes: []string{"accounts:read", "transfers:write"}, ExpiresAt: "2035-01-01T00:00:00Z"}},
		Accounts:    []provisioning.Account{{ID: "00000000-0000-0000-0000-000000000311", DisplayName: "Partner operating", Category: "operating", OpeningMinor: "0", ReadSubjects: []string{"partner-operator"}, DebitSubjects: []string{"partner-operator"}, CreditSubjects: []string{"partner-operator"}}},
	}
	repository, err := db.NewProvisioningRepository(database, func() time.Time { return time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	correlation := "00000000-0000-0000-0000-000000000302"
	if err = repository.Apply(ctx, configuration, "USD", "platform-owner", correlation); err != nil {
		t.Fatal(err)
	}
	if err = repository.Apply(ctx, configuration, "USD", "platform-owner", correlation); err != nil {
		t.Fatalf("safe retry failed: %v", err)
	}
	for name, query := range map[string]string{
		"policy":     `SELECT count(*) FROM tenant_transfer_policies WHERE tenant_id='00000000-0000-0000-0000-000000000301'`,
		"roles":      `SELECT count(*) FROM tenant_subject_roles WHERE tenant_id='00000000-0000-0000-0000-000000000301'`,
		"credential": `SELECT count(*) FROM partner_credential_events WHERE tenant_id='00000000-0000-0000-0000-000000000301' AND action='registered'`,
		"request":    `SELECT count(*) FROM partner_provisioning_requests WHERE tenant_id='00000000-0000-0000-0000-000000000301' AND status='applied'`,
		"audit":      `SELECT count(*) FROM audit_events WHERE tenant_id='00000000-0000-0000-0000-000000000301' AND event_type='partner.provisioned'`,
	} {
		if countRows(t, database, query) < 1 {
			t.Fatalf("missing provisioning %s evidence", name)
		}
	}
	if err = repository.Rollback(ctx, configuration.TenantID, "platform-reviewer", correlation); err != nil {
		t.Fatal(err)
	}
	if err = repository.Rollback(ctx, configuration.TenantID, "platform-reviewer", correlation); err != nil {
		t.Fatalf("safe rollback retry failed: %v", err)
	}
	if countRows(t, database, `SELECT count(*) FROM accounts WHERE tenant_id=$1 AND status='closed'`, configuration.TenantID) != 1 || countRows(t, database, `SELECT count(*) FROM partner_credential_events WHERE tenant_id=$1`, configuration.TenantID) != 2 {
		t.Fatal("rollback did not close accounts and append credential revocation")
	}
}
