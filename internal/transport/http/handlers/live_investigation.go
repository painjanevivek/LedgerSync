package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxLiveSignalBodyBytes = investigation.MaxLiveSignalBytes + 2048

type liveRoomStartRequest struct {
	RequestReference string `json:"request_reference"`
}
type liveRoomJoinRequest struct {
	InviteToken string `json:"invite_token"`
}
type liveRoomMutationRequest struct {
	ExpectedVersion string `json:"expected_version"`
}
type liveFindingRequest struct {
	ExpectedVersion string `json:"expected_version"`
	Code            string `json:"code"`
}
type liveSignalRequest struct {
	Sequence int64           `json:"sequence"`
	Kind     string          `json:"kind"`
	Payload  json.RawMessage `json:"payload"`
}

func (h *InvestigationHandler) TransferRequestStatus(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, false)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	reference, err := investigation.NormalizeRequestReference(request.PathValue("requestReference"))
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	status, err := repository.TransferRequestStatus(request.Context(), principal.TenantID, principal.SubjectID, reference)
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return
	}
	writeInvestigationJSON(writer, status)
}

func (h *InvestigationHandler) StartLiveRoom(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, true)
	if !ok {
		return
	}
	var input liveRoomStartRequest
	if err := decodeLiveJSON(writer, request, 4096, &input); err != nil {
		writeLiveDecodeError(writer, request, err)
		return
	}
	room, err := repository.StartLiveRoom(request.Context(), investigation.LiveRoomStart{TenantID: principal.TenantID, ActorID: principal.SubjectID, RequestReference: input.RequestReference, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"), OccurredAt: time.Now().UTC()})
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return
	}
	writer.Header().Set("Location", "/private/investigation/live-rooms/"+room.RoomID)
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(room)
}

func (h *InvestigationHandler) JoinLiveRoom(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, false)
	if !ok {
		return
	}
	var input liveRoomJoinRequest
	if err := decodeLiveJSON(writer, request, 4096, &input); err != nil || len(input.InviteToken) < 32 || len(input.InviteToken) > 128 {
		writeLiveDecodeError(writer, request, err)
		return
	}
	room, err := repository.JoinLiveRoom(request.Context(), investigation.LiveRoomJoin{TenantID: principal.TenantID, ActorID: principal.SubjectID, InviteToken: input.InviteToken, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"), OccurredAt: time.Now().UTC()})
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return
	}
	writeInvestigationJSON(writer, room)
}

func (h *InvestigationHandler) LiveRoom(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, false)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	room, ok := h.readAuthorizedLiveRoom(writer, request, repository, principal)
	if !ok {
		return
	}
	peerRole := "owner"
	if room.ParticipantRole == "owner" {
		peerRole = "peer"
	}
	present, err := h.liveSignals.IsPresent(request.Context(), room.RoomID, peerRole)
	if err != nil {
		httptransport.WriteError(writer, request, collaborationUnavailable())
		return
	}
	room.PeerPresent = present
	writeInvestigationJSON(writer, room)
}

func (h *InvestigationHandler) IssueLiveRoomInvite(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, true)
	if !ok {
		return
	}
	var input struct{}
	if err := decodeLiveJSON(writer, request, 512, &input); err != nil {
		writeLiveDecodeError(writer, request, err)
		return
	}
	room, ok := h.readAuthorizedLiveRoom(writer, request, repository, principal)
	if !ok || room.ParticipantRole != "owner" {
		if ok {
			httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		}
		return
	}
	invite, err := repository.IssueLiveRoomInvite(request.Context(), principal.TenantID, principal.SubjectID, room.RoomID, time.Now().UTC())
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return
	}
	writeInvestigationJSON(writer, map[string]any{"room_id": invite.RoomID, "invite_token": invite.Token, "expires_at": invite.ExpiresAt})
}

func (h *InvestigationHandler) LeaveLiveRoom(writer http.ResponseWriter, request *http.Request) {
	h.mutateLiveRoom(writer, request, false)
}
func (h *InvestigationHandler) EndLiveRoom(writer http.ResponseWriter, request *http.Request) {
	h.mutateLiveRoom(writer, request, true)
}

func (h *InvestigationHandler) mutateLiveRoom(writer http.ResponseWriter, request *http.Request, end bool) {
	principal, repository, ok := h.authorizeLive(writer, request, end)
	if !ok {
		return
	}
	var input liveRoomMutationRequest
	if err := decodeLiveJSON(writer, request, 4096, &input); err != nil {
		writeLiveDecodeError(writer, request, err)
		return
	}
	version, err := investigation.ParseWorkspaceVersion(input.ExpectedVersion)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	command := investigation.LiveRoomMutation{TenantID: principal.TenantID, ActorID: principal.SubjectID, RoomID: request.PathValue("roomId"), ExpectedVersion: version, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"), OccurredAt: time.Now().UTC()}
	var receipt investigation.LiveRoomReceipt
	if end {
		receipt, err = repository.EndLiveRoom(request.Context(), command)
	} else {
		receipt, err = repository.LeaveLiveRoom(request.Context(), command)
	}
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return
	}
	if end {
		_ = h.liveSignals.Clear(request.Context(), receipt.RoomID)
	}
	writeInvestigationJSON(writer, receipt)
}

func (h *InvestigationHandler) RecordLiveRoomFinding(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, true)
	if !ok {
		return
	}
	var input liveFindingRequest
	if err := decodeLiveJSON(writer, request, 4096, &input); err != nil {
		writeLiveDecodeError(writer, request, err)
		return
	}
	version, err := investigation.ParseWorkspaceVersion(input.ExpectedVersion)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	room, err := repository.RecordLiveRoomFinding(request.Context(), investigation.LiveFindingCommand{LiveRoomMutation: investigation.LiveRoomMutation{TenantID: principal.TenantID, ActorID: principal.SubjectID, RoomID: request.PathValue("roomId"), ExpectedVersion: version, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"), OccurredAt: time.Now().UTC()}, Code: input.Code})
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return
	}
	writeInvestigationJSON(writer, room)
}

func (h *InvestigationHandler) LiveRoomSignals(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, false)
	if !ok {
		return
	}
	room, ok := h.readAuthorizedLiveRoom(writer, request, repository, principal)
	if !ok {
		return
	}
	if request.Method == http.MethodGet {
		if !onlyQueryParameters(request, "cursor") {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
			return
		}
		page, err := h.liveSignals.Read(request.Context(), room.RoomID, request.URL.Query().Get("cursor"))
		if err != nil {
			httptransport.WriteError(writer, request, collaborationUnavailable())
			return
		}
		writeInvestigationJSON(writer, page)
		return
	}
	var input liveSignalRequest
	if err := decodeLiveJSON(writer, request, maxLiveSignalBodyBytes, &input); err != nil {
		writeLiveDecodeError(writer, request, err)
		return
	}
	signal := investigation.LiveSignal{RoomID: room.RoomID, Role: room.ParticipantRole, Sequence: input.Sequence, Kind: input.Kind, Payload: input.Payload, SentAt: time.Now().UTC()}
	if err := h.liveSignals.Append(request.Context(), signal); err != nil {
		if errors.Is(err, investigation.ErrInvalidLiveRoom) || errors.Is(err, investigation.ErrLiveRoomConflict) {
			httptransport.WriteError(writer, request, livePublicError(err))
		} else {
			httptransport.WriteError(writer, request, collaborationUnavailable())
		}
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (h *InvestigationHandler) LiveRoomPresence(writer http.ResponseWriter, request *http.Request) {
	principal, repository, ok := h.authorizeLive(writer, request, false)
	if !ok {
		return
	}
	var input struct{}
	if err := decodeLiveJSON(writer, request, 512, &input); err != nil {
		writeLiveDecodeError(writer, request, err)
		return
	}
	room, ok := h.readAuthorizedLiveRoom(writer, request, repository, principal)
	if !ok {
		return
	}
	if err := h.liveSignals.Presence(request.Context(), room.RoomID, room.ParticipantRole); err != nil {
		httptransport.WriteError(writer, request, collaborationUnavailable())
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (h *InvestigationHandler) readAuthorizedLiveRoom(writer http.ResponseWriter, request *http.Request, repository investigation.LiveRoomRepository, principal identity.Principal) (investigation.LiveRoom, bool) {
	roomID, err := investigation.NormalizeLiveRoomID(request.PathValue("roomId"))
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return investigation.LiveRoom{}, false
	}
	room, err := repository.GetLiveRoom(request.Context(), principal.TenantID, principal.SubjectID, roomID)
	if err != nil {
		httptransport.WriteError(writer, request, livePublicError(err))
		return investigation.LiveRoom{}, false
	}
	return room, true
}

func (h *InvestigationHandler) authorizeLive(writer http.ResponseWriter, request *http.Request, write bool) (identity.Principal, investigation.LiveRoomRepository, bool) {
	if h == nil || !h.liveEnabled || h.liveSignals == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return identity.Principal{}, nil, false
	}
	repository, configured := h.repository.(investigation.LiveRoomRepository)
	if !configured {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return identity.Principal{}, nil, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, nil, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:investigate")
		return identity.Principal{}, nil, false
	}
	for _, scope := range []string{"investigation:collaborate", "investigation:read", "transfers:read"} {
		if !principal.HasScope(scope) {
			writeScopeDenial(writer, request, h.audit, principal, scope)
			return identity.Principal{}, nil, false
		}
	}
	if write && !principal.HasScope("investigation:write") {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:write")
		return identity.Principal{}, nil, false
	}
	limit, route := h.rateLimit, "investigation:live-read"
	if write || request.Method != http.MethodGet {
		limit, route = h.workspaceWriteLimit, "investigation:live-write"
		if limit < 1 {
			limit = h.rateLimit
		}
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, limit, true) {
		return identity.Principal{}, nil, false
	}
	return principal, repository, true
}

func decodeLiveJSON(writer http.ResponseWriter, request *http.Request, limit int64, target any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errSavedViewMediaType
	}
	request.Body = http.MaxBytesReader(writer, request.Body, limit)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains more than one JSON value")
	}
	return nil
}

func writeLiveDecodeError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, errSavedViewMediaType) {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type must be application/json."})
		return
	}
	httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
}

func livePublicError(err error) error {
	switch {
	case errors.Is(err, investigation.ErrInvalidLiveRoom):
		return httptransport.ErrBadRequest
	case errors.Is(err, investigation.ErrLiveRoomNotFound), errors.Is(err, investigation.ErrLiveInviteExpired):
		return httptransport.ErrNotFound
	case errors.Is(err, investigation.ErrLiveRoomConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "investigation_room_conflict", Message: "The investigation room changed. Refresh it before retrying."}
	case errors.Is(err, investigation.ErrFindingConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "finding_conflicts_with_transfer", Message: "That finding does not match the authoritative transfer state."}
	default:
		return collaborationUnavailable()
	}
}

func collaborationUnavailable() error {
	return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "collaboration_unavailable", Message: "Live collaboration is unavailable. The transfer status is unchanged; use the normal status check."}
}

func parseLiveVersion(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}
