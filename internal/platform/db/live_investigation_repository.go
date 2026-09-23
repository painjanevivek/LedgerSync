package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
)

func (r *InvestigationRepository) TransferRequestStatus(ctx context.Context, tenantID, actorID, reference string) (investigation.TransferRequestStatus, error) {
	reference, err := investigation.NormalizeRequestReference(reference)
	if err != nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return investigation.TransferRequestStatus{}, investigation.ErrLiveRoomNotFound
	}
	var state, response, transferStatus string
	var expires time.Time
	err = r.database.QueryRowContext(ctx, `SELECT request.state,COALESCE(request.response_body::text,''),request.expires_at,COALESCE(transfer.status,'') FROM idempotency_requests request LEFT JOIN transfers transfer ON transfer.tenant_id=request.tenant_id AND transfer.id=CASE WHEN request.response_body ? 'transfer_id' AND request.response_body->>'transfer_id' ~ '^[0-9a-f-]{36}$' THEN (request.response_body->>'transfer_id')::uuid ELSE NULL END WHERE request.tenant_id=$1 AND request.actor_subject_id=$2 AND request.operation='transfers.create.v1' AND request.request_reference=$3`, tenantID, actorID, reference).Scan(&state, &response, &expires, &transferStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return investigation.TransferRequestStatus{}, investigation.ErrLiveRoomNotFound
	}
	if err != nil {
		return investigation.TransferRequestStatus{}, fmt.Errorf("read transfer request status: %w", err)
	}
	result := investigation.TransferRequestStatus{RequestReference: reference, Status: "unavailable", CheckedAt: time.Now().UTC()}
	var body struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	_ = json.Unmarshal([]byte(response), &body)
	if transferStatus == "posted" || body.Status == "posted" {
		result.Status, result.TransferID = "completed", strings.ToLower(body.TransferID)
	}
	if transferStatus == "rejected" || body.Status == "rejected" {
		result.Status, result.TransferID = "rejected", ""
	}
	if state == "in_progress" {
		result.Status = "in_progress"
		if !expires.After(result.CheckedAt) {
			result.Status = "expired"
		}
	}
	return result, nil
}

func (r *InvestigationRepository) StartLiveRoom(ctx context.Context, command investigation.LiveRoomStart) (investigation.LiveRoom, error) {
	ref, err := investigation.NormalizeRequestReference(command.RequestReference)
	if err != nil {
		return investigation.LiveRoom{}, err
	}
	command.RequestReference = ref
	when := liveTime(command.OccurredAt)
	fingerprint := sha256.Sum256([]byte(ref))
	var result investigation.LiveRoom
	err = r.withLiveOperation(ctx, command.TenantID, command.ActorID, "start", command.IdempotencyKey, fingerprint, &result, func(tx *sql.Tx) error {
		status, err := transferRequestStatusWithQuerier(ctx, tx, command.TenantID, command.ActorID, ref, when)
		if err != nil || status.Status != "in_progress" {
			return investigation.ErrLiveRoomNotFound
		}
		var expiredRoomID string
		var expiredVersion int64
		expireErr := tx.QueryRowContext(ctx, `UPDATE investigation_live_rooms SET status='expired',ended_at=$4,version=version+1,invite_token_hash=NULL,invite_expires_at=NULL WHERE tenant_id=$1 AND owner_subject_id=$2 AND transfer_request_reference=$3 AND status IN ('waiting','active') AND expires_at<=$4 RETURNING id::text,version`, command.TenantID, command.ActorID, ref, when).Scan(&expiredRoomID, &expiredVersion)
		if expireErr == nil {
			correlationID, _ := newUUID()
			if err := insertLiveAudit(ctx, tx, command.TenantID, command.ActorID, expiredRoomID, "investigation.live_room_expired", "expired", expiredVersion, correlationID, when); err != nil {
				return err
			}
		} else if !errors.Is(expireErr, sql.ErrNoRows) {
			return expireErr
		}
		if existing, found, err := readActiveRoomByReference(ctx, tx, command.TenantID, command.ActorID, ref, when); err != nil {
			return err
		} else if found {
			result = existing
			return nil
		}
		roomID, err := newUUID()
		if err != nil {
			return err
		}
		workspaceID, err := newUUID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO investigation_workspaces(id,tenant_id,owner_subject_id,title,taxonomy,status,query_kind,root_record_type,query_value,root_record_id,version,created_at,updated_at) VALUES($1,$2,$3,'Uncertain transfer request','transfer_delivery','open','request_reference','transfer_request',$4::text,$4::uuid,1,$5,$5)`, workspaceID, command.TenantID, command.ActorID, ref, when)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO investigation_workspace_references(investigation_id,tenant_id,position,relationship_type,record_type,record_id,captured_at) VALUES($1,$2,0,'root','transfer_request',$3,$4)`, workspaceID, command.TenantID, ref, when)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO investigation_live_rooms(id,tenant_id,investigation_id,transfer_request_reference,owner_subject_id,status,version,created_at,expires_at) VALUES($1,$2,$3,$4,$5,'waiting',1,$6,$7)`, roomID, command.TenantID, workspaceID, ref, command.ActorID, when, when.Add(investigation.LiveRoomLifetime))
		if err != nil {
			return err
		}
		result = investigation.LiveRoom{RoomID: roomID, InvestigationID: workspaceID, RequestReference: ref, Status: "waiting", ParticipantRole: "owner", Version: "1", CreatedAt: when, ExpiresAt: when.Add(investigation.LiveRoomLifetime), AuthoritativeState: status.Status}
		return insertLiveAudit(ctx, tx, command.TenantID, command.ActorID, roomID, "investigation.live_room_started", "waiting", 1, command.CorrelationID, when)
	})
	return result, mapLiveError(err)
}

func (r *InvestigationRepository) GetLiveRoom(ctx context.Context, tenantID, actorID, roomID string) (investigation.LiveRoom, error) {
	roomID, err := investigation.NormalizeLiveRoomID(roomID)
	if err != nil {
		return investigation.LiveRoom{}, investigation.ErrLiveRoomNotFound
	}
	return r.readLiveRoom(ctx, r.database, tenantID, actorID, roomID, time.Now().UTC())
}

func (r *InvestigationRepository) IssueLiveRoomInvite(ctx context.Context, tenantID, actorID, roomID string, when time.Time) (investigation.LiveInvite, error) {
	roomID, err := investigation.NormalizeLiveRoomID(roomID)
	if err != nil {
		return investigation.LiveInvite{}, investigation.ErrLiveRoomNotFound
	}
	when = liveTime(when)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return investigation.LiveInvite{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	digest := sha256.Sum256([]byte(token))
	result := investigation.LiveInvite{RoomID: roomID, Token: token, ExpiresAt: when.Add(investigation.LiveInviteLifetime)}
	err = WithSerializableSequence(ctx, r.database, "live-room:"+tenantID+":"+roomID, 3, func(tx *sql.Tx) error {
		var version int64
		err := tx.QueryRowContext(ctx, `UPDATE investigation_live_rooms SET invite_token_hash=$4,invite_expires_at=$5,version=version+1 WHERE tenant_id=$1 AND id=$2 AND owner_subject_id=$3 AND status IN ('waiting','active') AND expires_at>$6 RETURNING version`, tenantID, roomID, actorID, digest[:], result.ExpiresAt, when).Scan(&version)
		if errors.Is(err, sql.ErrNoRows) {
			return investigation.ErrLiveRoomNotFound
		}
		return err
	})
	return result, mapLiveError(err)
}

func (r *InvestigationRepository) JoinLiveRoom(ctx context.Context, command investigation.LiveRoomJoin) (investigation.LiveRoom, error) {
	when := liveTime(command.OccurredAt)
	digest := sha256.Sum256([]byte(strings.TrimSpace(command.InviteToken)))
	fingerprint := sha256.Sum256(digest[:])
	if len(strings.TrimSpace(command.InviteToken)) < 32 {
		return investigation.LiveRoom{}, investigation.ErrLiveRoomNotFound
	}
	var result investigation.LiveRoom
	err := r.withLiveOperation(ctx, command.TenantID, command.ActorID, "join", command.IdempotencyKey, fingerprint, &result, func(tx *sql.Tx) error {
		var roomID string
		err := tx.QueryRowContext(ctx, `SELECT id::text FROM investigation_live_rooms WHERE tenant_id=$1 AND invite_token_hash=$2 AND invite_expires_at>$3 AND expires_at>$3 AND status IN ('waiting','active') AND owner_subject_id<>$4 FOR UPDATE`, command.TenantID, digest[:], when, command.ActorID).Scan(&roomID)
		if errors.Is(err, sql.ErrNoRows) {
			return investigation.ErrLiveInviteExpired
		}
		if err != nil {
			return err
		}
		var version int64
		err = tx.QueryRowContext(ctx, `UPDATE investigation_live_rooms SET peer_subject_id=COALESCE(peer_subject_id,$4),status='active',invite_token_hash=NULL,invite_expires_at=NULL,version=version+1 WHERE tenant_id=$1 AND id=$2 AND (peer_subject_id IS NULL OR peer_subject_id=$3) RETURNING version`, command.TenantID, roomID, command.ActorID, command.ActorID).Scan(&version)
		if errors.Is(err, sql.ErrNoRows) {
			return investigation.ErrLiveRoomNotFound
		}
		if err != nil {
			return err
		}
		result, err = r.readLiveRoom(ctx, tx, command.TenantID, command.ActorID, roomID, when)
		if err != nil {
			return err
		}
		return insertLiveAudit(ctx, tx, command.TenantID, command.ActorID, roomID, "investigation.live_peer_joined", "active", version, command.CorrelationID, when)
	})
	return result, mapLiveError(err)
}

func (r *InvestigationRepository) LeaveLiveRoom(ctx context.Context, command investigation.LiveRoomMutation) (investigation.LiveRoomReceipt, error) {
	roomID, err := investigation.NormalizeLiveRoomID(command.RoomID)
	if err != nil {
		return investigation.LiveRoomReceipt{}, investigation.ErrLiveRoomNotFound
	}
	when := liveTime(command.OccurredAt)
	result := investigation.LiveRoomReceipt{RoomID: roomID, Outcome: "left", OccurredAt: when}
	err = WithSerializableSequence(ctx, r.database, "investigation-live-room:"+command.TenantID+":"+roomID, 3, func(tx *sql.Tx) error {
		var owner, peer, status string
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT owner_subject_id,COALESCE(peer_subject_id,''),status,version FROM investigation_live_rooms WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, command.TenantID, roomID).Scan(&owner, &peer, &status, &version)
		if errors.Is(err, sql.ErrNoRows) || command.ActorID != owner && command.ActorID != peer {
			return investigation.ErrLiveRoomNotFound
		}
		if err != nil {
			return err
		}
		if command.ExpectedVersion > 0 && command.ExpectedVersion != version {
			return investigation.ErrLiveRoomConflict
		}
		result.Version = strconv.FormatInt(version, 10)
		return insertLiveAudit(ctx, tx, command.TenantID, command.ActorID, roomID, "investigation.live_peer_left", status, version, command.CorrelationID, when)
	})
	return result, mapLiveError(err)
}

func (r *InvestigationRepository) EndLiveRoom(ctx context.Context, command investigation.LiveRoomMutation) (investigation.LiveRoomReceipt, error) {
	return r.liveRoomReceiptMutation(ctx, command, "end")
}

func (r *InvestigationRepository) liveRoomReceiptMutation(ctx context.Context, command investigation.LiveRoomMutation, action string) (investigation.LiveRoomReceipt, error) {
	roomID, err := investigation.NormalizeLiveRoomID(command.RoomID)
	if err != nil {
		return investigation.LiveRoomReceipt{}, investigation.ErrLiveRoomNotFound
	}
	when := liveTime(command.OccurredAt)
	fingerprint := sha256.Sum256([]byte(roomID + "|" + strconv.FormatInt(command.ExpectedVersion, 10)))
	result := investigation.LiveRoomReceipt{RoomID: roomID, Outcome: action + "d", OccurredAt: when}
	err = r.withLiveOperation(ctx, command.TenantID, command.ActorID, action, command.IdempotencyKey, fingerprint, &result, func(tx *sql.Tx) error {
		var owner, peer, status string
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT owner_subject_id,COALESCE(peer_subject_id,''),status,version FROM investigation_live_rooms WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, command.TenantID, roomID).Scan(&owner, &peer, &status, &version)
		if errors.Is(err, sql.ErrNoRows) || command.ActorID != owner && command.ActorID != peer {
			return investigation.ErrLiveRoomNotFound
		}
		if err != nil {
			return err
		}
		if action == "end" && command.ActorID != owner {
			return investigation.ErrLiveRoomNotFound
		}
		if command.ExpectedVersion > 0 && command.ExpectedVersion != version {
			return investigation.ErrLiveRoomConflict
		}
		if action == "end" && status != "ended" && status != "expired" {
			err = tx.QueryRowContext(ctx, `UPDATE investigation_live_rooms SET status='ended',ended_at=$3,version=version+1,invite_token_hash=NULL,invite_expires_at=NULL WHERE tenant_id=$1 AND id=$2 RETURNING version`, command.TenantID, roomID, when).Scan(&version)
			if err != nil {
				return err
			}
			status = "ended"
		}
		result.Version = strconv.FormatInt(version, 10)
		event := "investigation.live_peer_left"
		if action == "end" {
			event = "investigation.live_room_ended"
		}
		return insertLiveAudit(ctx, tx, command.TenantID, command.ActorID, roomID, event, status, version, command.CorrelationID, when)
	})
	return result, mapLiveError(err)
}

func (r *InvestigationRepository) RecordLiveRoomFinding(ctx context.Context, command investigation.LiveFindingCommand) (investigation.LiveRoom, error) {
	roomID, err := investigation.NormalizeLiveRoomID(command.RoomID)
	if err != nil {
		return investigation.LiveRoom{}, investigation.ErrLiveRoomNotFound
	}
	when := liveTime(command.OccurredAt)
	fingerprint := sha256.Sum256([]byte(roomID + "|" + command.Code))
	var result investigation.LiveRoom
	err = r.withLiveOperation(ctx, command.TenantID, command.ActorID, "finding", command.IdempotencyKey, fingerprint, &result, func(tx *sql.Tx) error {
		var owner, ref string
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT owner_subject_id,transfer_request_reference::text,version FROM investigation_live_rooms WHERE tenant_id=$1 AND id=$2 AND status IN ('waiting','active') AND expires_at>$3 FOR UPDATE`, command.TenantID, roomID, when).Scan(&owner, &ref, &version)
		if errors.Is(err, sql.ErrNoRows) || owner != command.ActorID {
			return investigation.ErrLiveRoomNotFound
		}
		if err != nil {
			return err
		}
		if command.ExpectedVersion > 0 && command.ExpectedVersion != version {
			return investigation.ErrLiveRoomConflict
		}
		status, err := transferRequestStatusWithQuerier(ctx, tx, command.TenantID, owner, ref, when)
		if err != nil {
			return err
		}
		if !investigation.FindingAllowed(command.Code, status.Status, status.TransferID) {
			return investigation.ErrFindingConflict
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO investigation_live_room_findings(room_id,tenant_id,finding_code,authoritative_request_state,canonical_transfer_id,actor_subject_id,recorded_at) VALUES($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7)`, roomID, command.TenantID, command.Code, status.Status, status.TransferID, command.ActorID, when)
		if err != nil {
			return investigation.ErrLiveRoomConflict
		}
		if err = tx.QueryRowContext(ctx, `UPDATE investigation_live_rooms SET version=version+1 WHERE tenant_id=$1 AND id=$2 RETURNING version`, command.TenantID, roomID).Scan(&version); err != nil {
			return err
		}
		result, err = r.readLiveRoom(ctx, tx, command.TenantID, command.ActorID, roomID, when)
		if err != nil {
			return err
		}
		return insertLiveAudit(ctx, tx, command.TenantID, command.ActorID, roomID, "investigation.live_finding_recorded", result.Status, version, command.CorrelationID, when)
	})
	return result, mapLiveError(err)
}

type liveQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (r *InvestigationRepository) readLiveRoom(ctx context.Context, q liveQuerier, tenantID, actorID, roomID string, when time.Time) (investigation.LiveRoom, error) {
	var room investigation.LiveRoom
	var owner, peer string
	var version int64
	var findingCode, findingState, findingTransfer string
	var findingAt sql.NullTime
	err := q.QueryRowContext(ctx, `SELECT room.id::text,room.investigation_id::text,room.transfer_request_reference::text,room.owner_subject_id,COALESCE(room.peer_subject_id,''),room.status,room.version,room.created_at,room.expires_at,COALESCE(finding.finding_code,''),COALESCE(finding.authoritative_request_state,''),COALESCE(finding.canonical_transfer_id::text,''),finding.recorded_at FROM investigation_live_rooms room LEFT JOIN investigation_live_room_findings finding ON finding.room_id=room.id WHERE room.tenant_id=$1 AND room.id=$2 AND (room.owner_subject_id=$3 OR room.peer_subject_id=$3)`, tenantID, roomID, actorID).Scan(&room.RoomID, &room.InvestigationID, &room.RequestReference, &owner, &peer, &room.Status, &version, &room.CreatedAt, &room.ExpiresAt, &findingCode, &findingState, &findingTransfer, &findingAt)
	if errors.Is(err, sql.ErrNoRows) {
		return investigation.LiveRoom{}, investigation.ErrLiveRoomNotFound
	}
	if err != nil {
		return investigation.LiveRoom{}, err
	}
	if room.ExpiresAt.Before(when) && room.Status != "ended" && room.Status != "expired" {
		var expiredVersion int64
		if updateErr := q.QueryRowContext(ctx, `UPDATE investigation_live_rooms SET status='expired',ended_at=$3,version=version+1,invite_token_hash=NULL,invite_expires_at=NULL WHERE tenant_id=$1 AND id=$2 AND status IN ('waiting','active') RETURNING version`, tenantID, roomID, when).Scan(&expiredVersion); updateErr == nil {
			room.Status, version = "expired", expiredVersion
			correlationID, _ := newUUID()
			_ = insertLiveAudit(ctx, q, tenantID, actorID, roomID, "investigation.live_room_expired", "expired", version, correlationID, when)
		}
	}
	room.ParticipantRole = "peer"
	if actorID == owner {
		room.ParticipantRole = "owner"
	}
	room.PeerBound = peer != ""
	room.Version = strconv.FormatInt(version, 10)
	room.CreatedAt = room.CreatedAt.UTC()
	room.ExpiresAt = room.ExpiresAt.UTC()
	status, err := transferRequestStatusWithQuerier(ctx, q, tenantID, owner, room.RequestReference, when)
	if err == nil {
		room.AuthoritativeState = status.Status
		room.TransferID = status.TransferID
	} else {
		room.AuthoritativeState = "unavailable"
	}
	if findingCode != "" {
		room.Finding = &investigation.LiveFinding{Code: findingCode, AuthoritativeRequestState: findingState, TransferID: findingTransfer, RecordedAt: findingAt.Time.UTC()}
	}
	return room, nil
}

func readActiveRoomByReference(ctx context.Context, q liveQuerier, tenantID, actorID, ref string, when time.Time) (investigation.LiveRoom, bool, error) {
	var id string
	err := q.QueryRowContext(ctx, `SELECT id::text FROM investigation_live_rooms WHERE tenant_id=$1 AND owner_subject_id=$2 AND transfer_request_reference=$3 AND status IN ('waiting','active') AND expires_at>$4`, tenantID, actorID, ref, when).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return investigation.LiveRoom{}, false, nil
	}
	if err != nil {
		return investigation.LiveRoom{}, false, err
	}
	room, err := (&InvestigationRepository{}).readLiveRoom(ctx, q, tenantID, actorID, id, when)
	return room, true, err
}

func transferRequestStatusWithQuerier(ctx context.Context, q liveQuerier, tenantID, actorID, ref string, when time.Time) (investigation.TransferRequestStatus, error) {
	var state, response string
	var expires time.Time
	err := q.QueryRowContext(ctx, `SELECT state,COALESCE(response_body::text,''),expires_at FROM idempotency_requests WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation='transfers.create.v1' AND request_reference=$3`, tenantID, actorID, ref).Scan(&state, &response, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return investigation.TransferRequestStatus{}, investigation.ErrLiveRoomNotFound
	}
	if err != nil {
		return investigation.TransferRequestStatus{}, err
	}
	result := investigation.TransferRequestStatus{RequestReference: ref, Status: "unavailable", CheckedAt: when}
	var body struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	_ = json.Unmarshal([]byte(response), &body)
	if state == "in_progress" {
		result.Status = "in_progress"
		if !expires.After(when) {
			result.Status = "expired"
		}
		return result, nil
	}
	if body.Status == "posted" {
		result.Status = "completed"
		result.TransferID = strings.ToLower(body.TransferID)
	} else if body.Status == "rejected" {
		result.Status = "rejected"
	}
	return result, nil
}

func (r *InvestigationRepository) withLiveOperation(ctx context.Context, tenantID, actorID, operation, key string, fingerprint [32]byte, result any, action func(*sql.Tx) error) error {
	key = strings.TrimSpace(key)
	if len(key) < 16 || len(key) > 255 {
		return investigation.ErrInvalidLiveRoom
	}
	for _, c := range key {
		if c < 0x21 || c > 0x7e {
			return investigation.ErrInvalidLiveRoom
		}
	}
	digest := sha256.Sum256([]byte(key))
	return WithSerializableSequence(ctx, r.database, "live-op:"+tenantID+":"+actorID+":"+operation+":"+hex.EncodeToString(digest[:]), 3, func(tx *sql.Tx) error {
		var storedFingerprint, body []byte
		err := tx.QueryRowContext(ctx, `SELECT request_fingerprint,response_body FROM investigation_live_room_operations WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key_hash=$4`, tenantID, actorID, operation, digest[:]).Scan(&storedFingerprint, &body)
		if err == nil {
			if !equalBytes(storedFingerprint, fingerprint[:]) {
				return investigation.ErrLiveRoomConflict
			}
			return json.Unmarshal(body, result)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err = action(tx); err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		roomID := liveRoomIDFromResult(result)
		id, err := newUUID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO investigation_live_room_operations(id,tenant_id,actor_subject_id,operation,idempotency_key_hash,request_fingerprint,room_id,response_body,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, tenantID, actorID, operation, digest[:], fingerprint[:], roomID, encoded, time.Now().UTC())
		return err
	})
}

func liveRoomIDFromResult(value any) string {
	switch v := value.(type) {
	case *investigation.LiveRoom:
		return v.RoomID
	case *investigation.LiveRoomReceipt:
		return v.RoomID
	default:
		return ""
	}
}
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}
func insertLiveAudit(ctx context.Context, tx liveQuerier, tenantID, actorID, roomID, event, status string, version int64, correlationID string, when time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	metadata, _ := json.Marshal(map[string]string{"status": status, "version": strconv.FormatInt(version, 10)})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES($1,$2,$3,$4,'investigation_live_room',$5,'succeeded',$6,$7,$8)`, id, tenantID, actorID, event, roomID, correlationID, metadata, when)
	return err
}
func liveTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
func mapLiveError(err error) error {
	if err == nil || errors.Is(err, investigation.ErrInvalidLiveRoom) || errors.Is(err, investigation.ErrLiveRoomNotFound) || errors.Is(err, investigation.ErrLiveRoomConflict) || errors.Is(err, investigation.ErrLiveInviteExpired) || errors.Is(err, investigation.ErrFindingConflict) {
		return err
	}
	return fmt.Errorf("live investigation repository: %w", err)
}

var _ investigation.LiveRoomRepository = (*InvestigationRepository)(nil)
