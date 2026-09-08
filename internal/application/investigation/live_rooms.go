package investigation

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	LiveRoomLifetime   = 30 * time.Minute
	LiveInviteLifetime = 10 * time.Minute
	LiveSignalTTL      = 35 * time.Minute
	MaxLiveSignals     = 200
	MaxLiveSignalBytes = 64 * 1024
)

var (
	ErrInvalidLiveRoom   = errors.New("invalid live investigation room")
	ErrLiveRoomNotFound  = errors.New("live investigation room not found")
	ErrLiveRoomConflict  = errors.New("live investigation room conflict")
	ErrLiveInviteExpired = errors.New("live investigation invite expired")
	ErrFindingConflict   = errors.New("investigation finding contradicts authoritative state")
	liveUUID             = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type TransferRequestStatus struct {
	RequestReference string    `json:"request_reference"`
	Status           string    `json:"status"`
	TransferID       string    `json:"transfer_id,omitempty"`
	CheckedAt        time.Time `json:"checked_at"`
}

type LiveFinding struct {
	Code                      string    `json:"code"`
	AuthoritativeRequestState string    `json:"authoritative_request_state"`
	TransferID                string    `json:"transfer_id,omitempty"`
	RecordedAt                time.Time `json:"recorded_at"`
}

type LiveRoom struct {
	RoomID             string       `json:"room_id"`
	InvestigationID    string       `json:"investigation_id"`
	RequestReference   string       `json:"request_reference"`
	Status             string       `json:"status"`
	ParticipantRole    string       `json:"participant_role"`
	PeerBound          bool         `json:"peer_bound"`
	PeerPresent        bool         `json:"peer_present"`
	Version            string       `json:"version"`
	CreatedAt          time.Time    `json:"created_at"`
	ExpiresAt          time.Time    `json:"expires_at"`
	AuthoritativeState string       `json:"authoritative_request_state"`
	TransferID         string       `json:"transfer_id,omitempty"`
	Finding            *LiveFinding `json:"finding,omitempty"`
}

type LiveRoomReceipt struct {
	RoomID     string    `json:"room_id"`
	Outcome    string    `json:"outcome"`
	Version    string    `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	InviteURL  string    `json:"invite_url,omitempty"`
}

type LiveRoomStart struct {
	TenantID, ActorID, RequestReference, CorrelationID, IdempotencyKey string
	OccurredAt                                                         time.Time
}

type LiveRoomJoin struct {
	TenantID, ActorID, InviteToken, CorrelationID, IdempotencyKey string
	OccurredAt                                                    time.Time
}

type LiveRoomMutation struct {
	TenantID, ActorID, RoomID, CorrelationID, IdempotencyKey string
	ExpectedVersion                                          int64
	OccurredAt                                               time.Time
}

type LiveFindingCommand struct {
	LiveRoomMutation
	Code string
}

type LiveInvite struct {
	RoomID    string
	Token     string
	ExpiresAt time.Time
}

type LiveRoomRepository interface {
	TransferRequestStatus(context.Context, string, string, string) (TransferRequestStatus, error)
	StartLiveRoom(context.Context, LiveRoomStart) (LiveRoom, error)
	GetLiveRoom(context.Context, string, string, string) (LiveRoom, error)
	IssueLiveRoomInvite(context.Context, string, string, string, time.Time) (LiveInvite, error)
	JoinLiveRoom(context.Context, LiveRoomJoin) (LiveRoom, error)
	LeaveLiveRoom(context.Context, LiveRoomMutation) (LiveRoomReceipt, error)
	EndLiveRoom(context.Context, LiveRoomMutation) (LiveRoomReceipt, error)
	RecordLiveRoomFinding(context.Context, LiveFindingCommand) (LiveRoom, error)
}

type LiveSignal struct {
	ID       string          `json:"id"`
	RoomID   string          `json:"room_id"`
	Role     string          `json:"role"`
	Sequence int64           `json:"sequence"`
	Kind     string          `json:"kind"`
	Payload  json.RawMessage `json:"payload"`
	SentAt   time.Time       `json:"sent_at"`
}

type LiveSignalPage struct {
	Signals []LiveSignal `json:"signals"`
	Cursor  string       `json:"cursor"`
}

type LiveSignalStore interface {
	Append(context.Context, LiveSignal) error
	Read(context.Context, string, string) (LiveSignalPage, error)
	Presence(context.Context, string, string) error
	IsPresent(context.Context, string, string) (bool, error)
	Clear(context.Context, string) error
}

func NormalizeRequestReference(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !liveUUID.MatchString(value) {
		return "", ErrInvalidLiveRoom
	}
	return value, nil
}

func NormalizeLiveRoomID(value string) (string, error) { return NormalizeRequestReference(value) }

func FindingAllowed(code, state, transferID string) bool {
	switch code {
	case "confirmed_completed":
		return state == "completed" && liveUUID.MatchString(strings.ToLower(transferID))
	case "confirmed_rejected":
		return state == "rejected" && transferID == ""
	case "still_unresolved", "escalated":
		return (state == "in_progress" || state == "unavailable" || state == "expired") && transferID == ""
	default:
		return false
	}
}

func NormalizeLiveSignal(signal LiveSignal) (LiveSignal, error) {
	signal.RoomID = strings.ToLower(strings.TrimSpace(signal.RoomID))
	signal.Role = strings.TrimSpace(signal.Role)
	signal.Kind = strings.TrimSpace(signal.Kind)
	if !liveUUID.MatchString(signal.RoomID) || (signal.Role != "owner" && signal.Role != "peer") || signal.Sequence < 1 || len(signal.Payload) == 0 || len(signal.Payload) > MaxLiveSignalBytes {
		return LiveSignal{}, ErrInvalidLiveRoom
	}
	switch signal.Kind {
	case "offer", "answer":
		var value struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		if json.Unmarshal(signal.Payload, &value) != nil || value.Type != signal.Kind || value.SDP == "" || len(value.SDP) > MaxLiveSignalBytes-256 {
			return LiveSignal{}, ErrInvalidLiveRoom
		}
	case "ice":
		var value struct {
			Candidate string `json:"candidate"`
		}
		if json.Unmarshal(signal.Payload, &value) != nil || value.Candidate == "" {
			return LiveSignal{}, ErrInvalidLiveRoom
		}
	case "end":
		if string(signal.Payload) != `{}` {
			return LiveSignal{}, ErrInvalidLiveRoom
		}
	default:
		return LiveSignal{}, ErrInvalidLiveRoom
	}
	return signal, nil
}
