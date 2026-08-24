package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/guidance"
	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

type integrationGuidanceRecovery struct {
	snapshot apprecovery.ManifestSnapshot
	err      error
}

func (s integrationGuidanceRecovery) Snapshot(context.Context) (apprecovery.ManifestSnapshot, error) {
	return s.snapshot, s.err
}

func TestGuidanceTimelineProvesCoverageOnlyFromVisibleStoredWatermark(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	reconciliationRepository, err := db.NewReconciliationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	before, err := reconciliationRepository.Reconcile(context.Background(), testTenantID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	submission, err := transferService.Submit(context.Background(), transferCommand(t, "phase8-guidance-key-0001", "25.00"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewGuidanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := guidance.NewService(repository, integrationGuidanceRecovery{err: errors.New("recovery unavailable")}, nil)
	if err != nil {
		t.Fatal(err)
	}

	initial, err := service.ExplainTransfer(context.Background(), testTenantID, testActorID, submission.Result.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Stages[0].State != "available" || initial.Stages[1].State != "available" || initial.Stages[2].State != "available" || initial.Stages[3].State != "available" || initial.Stages[4].State != "available" {
		t.Fatalf("committed transfer links missing: %+v", initial.Stages)
	}
	if initial.Stages[5].State != "missing" || initial.Stages[6].State != "missing" || initial.Stages[6].ReasonCode != "coverage_not_provable" {
		t.Fatalf("absent delivery/pre-transfer reconciliation was inferred: %+v", initial.Stages)
	}
	for _, item := range initial.Stages[6].Evidence {
		if item.EvidenceID == before.ID {
			t.Fatalf("pre-transfer reconciliation %s falsely covered transfer", before.ID)
		}
	}

	after, err := reconciliationRepository.Reconcile(context.Background(), testTenantID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	covered, err := service.ExplainTransfer(context.Background(), testTenantID, testActorID, submission.Result.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	if covered.Stages[6].State != "available" || len(covered.Stages[6].Evidence) != 1 || covered.Stages[6].Evidence[0].EvidenceID != after.ID {
		t.Fatalf("post-transfer stored watermark coverage missing: %+v", covered.Stages[6])
	}

	var eventID string
	if err := database.QueryRowContext(context.Background(), `SELECT id::text FROM outbox_events WHERE tenant_id=$1 AND transfer_id=$2 ORDER BY id LIMIT 1`, testTenantID, submission.Result.TransferID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(context.Background(), `INSERT INTO delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,due_at,completed_at) VALUES('70000000-0000-4000-8000-000000000091',$1,$2,$3,'webhook','private-endpoint',1,'delivered',now(),now())`, testTenantID, submission.Result.TransferID, eventID); err != nil {
		t.Fatal(err)
	}
	withDelivery, err := service.ExplainTransfer(context.Background(), testTenantID, testActorID, submission.Result.TransferID)
	if err != nil || withDelivery.Stages[5].State != "available" || len(withDelivery.Stages[5].Evidence) != 1 || withDelivery.Stages[5].Evidence[0].Status != "delivered" {
		t.Fatalf("delivery evidence=%+v error=%v", withDelivery.Stages[5], err)
	}
}

func TestGuidancePostgreSQLAuthorizationAndOrientationTruth(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	submission, err := transferService.Submit(context.Background(), transferCommand(t, "phase8-guidance-key-0002", "10.00"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewGuidanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	service, _ := guidance.NewService(repository, integrationGuidanceRecovery{snapshot: apprecovery.ManifestSnapshot{LatestBackup: &apprecovery.BackupManifestEvidence{BackupID: "backup-20260825T120000Z-abcdef0", FinalizedAtUTC: "2026-08-25T12:00:00Z"}}}, nil)

	if _, err := service.ExplainTransfer(context.Background(), testTenantID, "unrelated-subject", submission.Result.TransferID); !errors.Is(err, guidance.ErrTransferNotFound) {
		t.Fatalf("unrelated actor received transfer evidence: %v", err)
	}
	if _, err := service.ExplainTransfer(context.Background(), "00000000-0000-4000-8000-000000000099", testActorID, submission.Result.TransferID); !errors.Is(err, guidance.ErrTransferNotFound) {
		t.Fatalf("cross-tenant actor received transfer evidence: %v", err)
	}

	orientation, err := service.Orientation(context.Background(), testTenantID, testActorID)
	if err != nil || len(orientation.Steps) != 7 {
		t.Fatalf("orientation=%+v error=%v", orientation, err)
	}
	if orientation.Steps[0].State != "evidence_available" || orientation.Steps[0].ReasonCode != "browser_action_not_recorded" || orientation.Steps[2].State != "completed" || orientation.Steps[3].State != "evidence_available" || orientation.Steps[6].State != "completed" {
		t.Fatalf("orientation fabricated or omitted durable progress: %+v", orientation.Steps)
	}
	if orientation.Steps[1].State != "missing" || orientation.Steps[4].State != "missing" || orientation.Steps[5].State != "missing" {
		t.Fatalf("unstored actions were marked complete: %+v", orientation.Steps)
	}
	if _, err := database.ExecContext(context.Background(), `UPDATE outbox_events SET event_type='supersecret' WHERE id=(SELECT id FROM outbox_events WHERE tenant_id=$1 AND transfer_id=$2 ORDER BY id LIMIT 1)`, testTenantID, submission.Result.TransferID); err != nil {
		t.Fatal(err)
	}
	sanitized, err := service.ExplainTransfer(context.Background(), testTenantID, testActorID, submission.Result.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range sanitized.Stages[4].Evidence {
		if item.EventType == "supersecret" {
			t.Fatalf("unapproved event type leaked: %+v", item)
		}
	}
}
