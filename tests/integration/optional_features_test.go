package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestPortfolioOptionalFeatures(t *testing.T) {
	if os.Getenv("LEDGERSYNC_RUN_OPTIONAL") != "true" {
		t.Skip("requires guarded optional-feature profile")
	}
	portfolioControlDirectory(t)
	t.Run("restricted-cron-worker", TestCronWorkerRequiresSeparateRestrictedLogin)
	t.Run("retained-room-history", TestLiveRoomHistoryDoesNotExpireOrWriteEvidence)
	t.Run("enabled-collaboration", TestLiveInvestigationRoomLifecycleIsTenantScopedAndAudited)
	t.Run("ephemeral-signals", TestLiveInvestigationSignalsAreOrderedBoundedAndEphemeral)
	t.Run("scheduler-fencing", TestSchedulerLeaseTakeoverRejectsStaleJobs)
	t.Run("scheduler-tenant-isolation", TestSchedulerIdentityCannotReadOrWriteOtherTenants)
}

func TestLiveRoomHistoryDoesNotExpireOrWriteEvidence(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	when := time.Now().UTC()
	if _, err := admin.ExecContext(ctx, `INSERT INTO idempotency_requests(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,expires_at,created_at,request_reference) VALUES($1,$2,'transfers.create.v1','history-transfer-000034',decode(repeat('ab',32),'hex'),'in_progress',$3,$4,$5)`, testTenantID, testActorID, when.Add(time.Hour), when, liveRequestReference); err != nil {
		t.Fatal(err)
	}
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewInvestigationRepository(api.db)
	room, err := repository.StartLiveRoom(ctx, investigation.LiveRoomStart{TenantID: testTenantID, ActorID: testActorID, RequestReference: liveRequestReference, CorrelationID: liveCorrelationID, IdempotencyKey: "retained-history-start", OccurredAt: when})
	if err != nil {
		t.Fatal(err)
	}
	found, err := repository.RecordLiveRoomFinding(ctx, investigation.LiveFindingCommand{LiveRoomMutation: investigation.LiveRoomMutation{TenantID: testTenantID, ActorID: testActorID, RoomID: room.RoomID, CorrelationID: liveCorrelationID, IdempotencyKey: "retained-history-finding", ExpectedVersion: mustParseTestVersion(t, room.Version), OccurredAt: when}, Code: "still_unresolved"})
	if err != nil {
		t.Fatal(err)
	}
	// Move both timestamps together. The stored lifecycle state remains intact.
	if _, err := admin.ExecContext(ctx, `UPDATE investigation_live_rooms SET expires_at=expires_at-interval '1 hour',created_at=created_at-interval '1 hour' WHERE id=$1`, room.RoomID); err != nil {
		t.Fatal(err)
	}
	before := liveMigrationSnapshot(t, admin)
	auditBefore := countRows(t, admin, `SELECT count(*) FROM audit_events`)
	for i := 0; i < 2; i++ {
		history, err := repository.GetLiveRoomHistory(ctx, testTenantID, testActorID, room.RoomID)
		if err != nil || history.Finding == nil || history.Finding.Code != "still_unresolved" || history.Version != found.Version || history.Status != found.Status || history.PeerPresent {
			t.Fatal("retained authorized history changed", err)
		}
	}
	for _, denied := range []struct{ tenant, actor string }{{testTenantID, "unrelated-operator"}, {"00000000-0000-4000-8000-000000009999", testActorID}} {
		if _, err := repository.GetLiveRoomHistory(ctx, denied.tenant, denied.actor, room.RoomID); !errors.Is(err, investigation.ErrLiveRoomNotFound) {
			t.Fatal("history escaped participant/tenant scope", err)
		}
	}
	if after := liveMigrationSnapshot(t, admin); before != after || countRows(t, admin, `SELECT count(*) FROM audit_events`) != auditBefore {
		t.Fatal("history read mutated stored financial, room or audit evidence")
	}
}
