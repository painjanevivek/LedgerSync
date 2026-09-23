package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

const liveTestRoomID = "12345678-1234-4234-8234-123456789012"
const liveTestWorkspaceID = "12345678-1234-4234-8234-123456789013"
const liveTestReference = "12345678-1234-4234-8234-123456789014"

type liveRepositoryStub struct {
	investigation.Repository
	started investigation.LiveRoomStart
}

func (r *liveRepositoryStub) TransferRequestStatus(context.Context, string, string, string) (investigation.TransferRequestStatus, error) {
	return investigation.TransferRequestStatus{RequestReference: liveTestReference, Status: "in_progress", CheckedAt: time.Now().UTC()}, nil
}
func (r *liveRepositoryStub) StartLiveRoom(_ context.Context, command investigation.LiveRoomStart) (investigation.LiveRoom, error) {
	r.started = command
	return investigation.LiveRoom{RoomID: liveTestRoomID, InvestigationID: liveTestWorkspaceID, RequestReference: liveTestReference, Status: "waiting", ParticipantRole: "owner", Version: "1", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(30 * time.Minute), AuthoritativeState: "in_progress"}, nil
}
func (r *liveRepositoryStub) GetLiveRoom(context.Context, string, string, string) (investigation.LiveRoom, error) {
	return investigation.LiveRoom{}, investigation.ErrLiveRoomNotFound
}
func (r *liveRepositoryStub) IssueLiveRoomInvite(context.Context, string, string, string, time.Time) (investigation.LiveInvite, error) {
	return investigation.LiveInvite{}, nil
}
func (r *liveRepositoryStub) JoinLiveRoom(context.Context, investigation.LiveRoomJoin) (investigation.LiveRoom, error) {
	return investigation.LiveRoom{}, nil
}
func (r *liveRepositoryStub) LeaveLiveRoom(context.Context, investigation.LiveRoomMutation) (investigation.LiveRoomReceipt, error) {
	return investigation.LiveRoomReceipt{}, nil
}
func (r *liveRepositoryStub) EndLiveRoom(context.Context, investigation.LiveRoomMutation) (investigation.LiveRoomReceipt, error) {
	return investigation.LiveRoomReceipt{}, nil
}
func (r *liveRepositoryStub) RecordLiveRoomFinding(context.Context, investigation.LiveFindingCommand) (investigation.LiveRoom, error) {
	return investigation.LiveRoom{}, nil
}

type liveSignalStub struct{}

func (liveSignalStub) Append(context.Context, investigation.LiveSignal) error { return nil }
func (liveSignalStub) Read(context.Context, string, string) (investigation.LiveSignalPage, error) {
	return investigation.LiveSignalPage{Signals: []investigation.LiveSignal{}, Cursor: "0-0"}, nil
}
func (liveSignalStub) Presence(context.Context, string, string) error          { return nil }
func (liveSignalStub) IsPresent(context.Context, string, string) (bool, error) { return false, nil }
func (liveSignalStub) Clear(context.Context, string) error                     { return nil }

func livePrincipal(scopes ...string) fixedInvestigationPrincipal {
	grants := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		grants[scope] = struct{}{}
	}
	return fixedInvestigationPrincipal{principal: identity.Principal{SubjectID: "operator-1", TenantID: "tenant-1", Roles: map[string]struct{}{"tenant:operator": {}}, Scopes: grants}}
}

func TestLiveInvestigationFeatureFailsClosedWhenDisabled(t *testing.T) {
	handler := NewInvestigationHandler(&liveRepositoryStub{}, livePrincipal("investigation:collaborate", "investigation:read", "transfers:read"))
	request := httptest.NewRequest(http.MethodGet, "/private/transfer-requests/"+liveTestReference+"/status", nil)
	request.SetPathValue("requestReference", liveTestReference)
	response := httptest.NewRecorder()
	handler.TransferRequestStatus(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestLiveInvestigationRequiresDedicatedScope(t *testing.T) {
	handler := NewInvestigationHandler(&liveRepositoryStub{}, livePrincipal("investigation:read", "transfers:read")).WithLiveCollaboration(true, liveSignalStub{})
	request := httptest.NewRequest(http.MethodGet, "/private/transfer-requests/"+liveTestReference+"/status", nil)
	request.SetPathValue("requestReference", liveTestReference)
	response := httptest.NewRecorder()
	handler.TransferRequestStatus(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStartLiveInvestigationBindsAuthenticatedActorAndReference(t *testing.T) {
	repository := &liveRepositoryStub{}
	handler := NewInvestigationHandler(repository, livePrincipal("investigation:collaborate", "investigation:read", "investigation:write", "transfers:read")).WithLiveCollaboration(true, liveSignalStub{})
	request := httptest.NewRequest(http.MethodPost, "/private/investigation/live-rooms", strings.NewReader(`{"request_reference":"`+liveTestReference+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "live-room-start-0001")
	response := httptest.NewRecorder()
	handler.StartLiveRoom(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repository.started.TenantID != "tenant-1" || repository.started.ActorID != "operator-1" || repository.started.RequestReference != liveTestReference {
		t.Fatalf("unexpected command: %#v", repository.started)
	}
	var result investigation.LiveRoom
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.RoomID != liveTestRoomID {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
