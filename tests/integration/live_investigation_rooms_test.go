package integration_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

const (
	liveRequestReference = "00000000-0000-4000-8000-000000000034"
	liveCorrelationID    = "00000000-0000-4000-8000-000000000035"
)

func TestLiveInvestigationRoomLifecycleIsTenantScopedAndAudited(t *testing.T) {
	_, database := requireTransferService(t, 100_000)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if _, err := database.ExecContext(ctx, `
INSERT INTO idempotency_requests (
    tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,
    state,expires_at,created_at,request_reference
) VALUES ($1,$2,'transfers.create.v1','uncertain-transfer-000034',decode(repeat('ab',32),'hex'),
    'in_progress',$3,$4,$5)`, testTenantID, testActorID, time.Now().UTC().Add(time.Hour), time.Now().UTC(), liveRequestReference); err != nil {
		t.Fatalf("seed uncertain transfer request: %v", err)
	}

	repository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatalf("create investigation repository: %v", err)
	}
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	room, err := repository.StartLiveRoom(ctx, investigation.LiveRoomStart{
		TenantID: testTenantID, ActorID: testActorID, RequestReference: liveRequestReference,
		CorrelationID: liveCorrelationID, IdempotencyKey: "start-live-room-000034", OccurredAt: startedAt,
	})
	if err != nil {
		t.Fatalf("start live room: %v", err)
	}
	if room.ParticipantRole != "owner" || room.Status != "waiting" || room.RequestReference != liveRequestReference {
		t.Fatalf("unexpected started room: %#v", room)
	}
	replayed, err := repository.StartLiveRoom(ctx, investigation.LiveRoomStart{
		TenantID: testTenantID, ActorID: testActorID, RequestReference: liveRequestReference,
		CorrelationID: liveCorrelationID, IdempotencyKey: "start-live-room-000034", OccurredAt: startedAt,
	})
	if err != nil || replayed.RoomID != room.RoomID {
		t.Fatalf("idempotent room replay=%#v error=%v", replayed, err)
	}

	invite, err := repository.IssueLiveRoomInvite(ctx, testTenantID, testActorID, room.RoomID, startedAt.Add(time.Second))
	if err != nil || invite.Token == "" {
		t.Fatalf("issue live room invite=%#v error=%v", invite, err)
	}
	peerRoom, err := repository.JoinLiveRoom(ctx, investigation.LiveRoomJoin{
		TenantID: testTenantID, ActorID: "integration-peer", InviteToken: invite.Token,
		CorrelationID: liveCorrelationID, IdempotencyKey: "join-live-room-000034", OccurredAt: startedAt.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("join live room: %v", err)
	}
	if peerRoom.ParticipantRole != "peer" || !peerRoom.PeerBound || peerRoom.Status != "active" {
		t.Fatalf("unexpected peer room: %#v", peerRoom)
	}
	if _, err := repository.JoinLiveRoom(ctx, investigation.LiveRoomJoin{
		TenantID: testTenantID, ActorID: "integration-third", InviteToken: invite.Token,
		CorrelationID: liveCorrelationID, IdempotencyKey: "third-live-room-000034", OccurredAt: startedAt.Add(3 * time.Second),
	}); !errors.Is(err, investigation.ErrLiveInviteExpired) {
		t.Fatalf("used invite must not admit a third identity: %v", err)
	}
	if _, err := repository.GetLiveRoom(ctx, "00000000-0000-0000-0000-000000000099", testActorID, room.RoomID); !errors.Is(err, investigation.ErrLiveRoomNotFound) {
		t.Fatalf("cross-tenant read must be non-disclosing: %v", err)
	}

	ownerRoom, err := repository.GetLiveRoom(ctx, testTenantID, testActorID, room.RoomID)
	if err != nil {
		t.Fatalf("refresh owner room: %v", err)
	}
	if _, err := repository.RecordLiveRoomFinding(ctx, investigation.LiveFindingCommand{
		LiveRoomMutation: investigation.LiveRoomMutation{
			TenantID: testTenantID, ActorID: testActorID, RoomID: room.RoomID,
			CorrelationID: liveCorrelationID, IdempotencyKey: "finding-stale-000034",
			ExpectedVersion: 1, OccurredAt: startedAt.Add(4 * time.Second),
		}, Code: "still_unresolved",
	}); !errors.Is(err, investigation.ErrLiveRoomConflict) {
		t.Fatalf("stale finding version must conflict: %v", err)
	}
	found, err := repository.RecordLiveRoomFinding(ctx, investigation.LiveFindingCommand{
		LiveRoomMutation: investigation.LiveRoomMutation{
			TenantID: testTenantID, ActorID: testActorID, RoomID: room.RoomID,
			CorrelationID: liveCorrelationID, IdempotencyKey: "finding-live-room-000034",
			ExpectedVersion: mustParseTestVersion(t, ownerRoom.Version), OccurredAt: startedAt.Add(5 * time.Second),
		}, Code: "still_unresolved",
	})
	if err != nil || found.Finding == nil || found.Finding.Code != "still_unresolved" {
		t.Fatalf("record structured finding=%#v error=%v", found, err)
	}

	if countRows(t, database, `SELECT count(*) FROM investigation_live_room_findings WHERE room_id=$1`, room.RoomID) != 1 {
		t.Fatal("finding must be append-only and recorded once")
	}
	if countRows(t, database, `SELECT count(*) FROM audit_events WHERE target_type='investigation_live_room' AND target_id=$1 AND sanitized_metadata ?| array['invite_token','request_reference','actor_subject_id','sdp','ice','message']`, room.RoomID) != 0 {
		t.Fatal("audit metadata contains forbidden collaboration material")
	}
}

func mustParseTestVersion(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		t.Fatalf("invalid room version %q", value)
	}
	return parsed
}
