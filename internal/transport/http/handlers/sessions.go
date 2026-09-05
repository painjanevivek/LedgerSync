package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

const sessionRequestLimit = 16 << 10

type SessionHandler struct {
	repository sessionStore
	identity   identity.Provider
}

type sessionStore interface {
	Create(context.Context, db.SessionRecord, string) (string, error)
	Resolve(context.Context, string) (db.SessionRecord, error)
	Revoke(context.Context, string) error
	UpdateConsistency(context.Context, string, map[string]string) error
}

type sessionRequest struct {
	Token                   string            `json:"token,omitempty"`
	RotateToken             string            `json:"rotate_token,omitempty"`
	SubjectID               string            `json:"subject_id,omitempty"`
	TenantID                string            `json:"tenant_id,omitempty"`
	CSRFToken               string            `json:"csrf_token,omitempty"`
	ExpiresAt               int64             `json:"expires_at,omitempty"`
	AuthenticatedAt         int64             `json:"authenticated_at,omitempty"`
	Roles                   []string          `json:"roles,omitempty"`
	Scopes                  []string          `json:"scopes,omitempty"`
	ConsistencyRequirements map[string]string `json:"consistency_requirements,omitempty"`
}

type sessionResponse struct {
	Token                   string            `json:"token,omitempty"`
	SubjectID               string            `json:"subject_id,omitempty"`
	TenantID                string            `json:"tenant_id,omitempty"`
	CSRFToken               string            `json:"csrf_token,omitempty"`
	ExpiresAt               int64             `json:"expires_at,omitempty"`
	AuthenticatedAt         int64             `json:"authenticated_at,omitempty"`
	Roles                   []string          `json:"roles,omitempty"`
	Scopes                  []string          `json:"scopes,omitempty"`
	ConsistencyRequirements map[string]string `json:"consistency_requirements,omitempty"`
}

func NewSessionHandler(repository sessionStore, provider identity.Provider) *SessionHandler {
	return &SessionHandler{repository: repository, identity: provider}
}

func (h *SessionHandler) Create(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request) {
		return
	}
	input, ok := decodeSessionRequest(writer, request)
	if !ok || !validSessionClaims(request.Context(), input) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	record := db.SessionRecord{SubjectID: input.SubjectID, TenantID: input.TenantID, CSRFToken: input.CSRFToken, ExpiresAt: time.UnixMilli(input.ExpiresAt).UTC(), Roles: input.Roles, Scopes: input.Scopes, ConsistencyRequirements: map[string]string{}}
	if input.AuthenticatedAt > 0 {
		value := time.UnixMilli(input.AuthenticatedAt).UTC()
		record.AuthenticatedAt = &value
	}
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	token, err := h.repository.Create(ctx, record, input.RotateToken)
	if err != nil {
		writeSessionError(writer, request, err)
		return
	}
	writeSessionJSON(writer, http.StatusCreated, sessionResponse{Token: token})
}

func (h *SessionHandler) Resolve(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request) {
		return
	}
	input, ok := decodeSessionRequest(writer, request)
	if !ok || input.Token == "" {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	record, err := h.repository.Resolve(ctx, input.Token)
	if err != nil {
		writeSessionError(writer, request, err)
		return
	}
	response := sessionResponse{SubjectID: record.SubjectID, TenantID: record.TenantID, CSRFToken: record.CSRFToken, ExpiresAt: record.ExpiresAt.UnixMilli(), Roles: record.Roles, Scopes: record.Scopes, ConsistencyRequirements: record.ConsistencyRequirements}
	if record.AuthenticatedAt != nil {
		response.AuthenticatedAt = record.AuthenticatedAt.UnixMilli()
	}
	writeSessionJSON(writer, http.StatusOK, response)
}

func (h *SessionHandler) Revoke(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request) {
		return
	}
	input, ok := decodeSessionRequest(writer, request)
	if !ok || input.Token == "" {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	if err := h.repository.Revoke(ctx, input.Token); err != nil && !errors.Is(err, db.ErrSessionNotFound) {
		writeSessionError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (h *SessionHandler) UpdateConsistency(writer http.ResponseWriter, request *http.Request) {
	if !h.authorize(writer, request) {
		return
	}
	input, ok := decodeSessionRequest(writer, request)
	if !ok || input.Token == "" || !validRequirements(input.ConsistencyRequirements) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	if err := h.repository.UpdateConsistency(ctx, input.Token, input.ConsistencyRequirements); err != nil {
		writeSessionError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (h *SessionHandler) authorize(writer http.ResponseWriter, request *http.Request) bool {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("session handler is not configured"))
		return false
	}
	principal, err := h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return false
	}
	if identity.RequireScope(principal, identity.BFFActorScope) != nil {
		writeScopeDenial(writer, request, nil, principal, identity.BFFActorScope)
		return false
	}
	return true
}

func decodeSessionRequest(writer http.ResponseWriter, request *http.Request) (sessionRequest, bool) {
	if request.Header.Get("Content-Type") != "application/json" {
		return sessionRequest{}, false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, sessionRequestLimit)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input sessionRequest
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return sessionRequest{}, false
	}
	return input, true
}

func validSessionClaims(ctx context.Context, input sessionRequest) bool {
	if len(input.SubjectID) < 1 || len(input.SubjectID) > 256 || len(input.CSRFToken) < 16 || len(input.CSRFToken) > 128 || len(input.Roles) > 16 || len(input.Scopes) > 32 || input.ExpiresAt <= time.Now().UnixMilli() || input.ExpiresAt > time.Now().Add(31*time.Minute).UnixMilli() {
		return false
	}
	if _, err := identifier.Parse(ctx, identifier.KindTenant, input.TenantID); err != nil {
		return false
	}
	return validSessionStrings(input.Roles) && validSessionStrings(input.Scopes)
}

func validSessionStrings(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		if strings.TrimSpace(value) != value || len(value) < 1 || len(value) > 64 {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validRequirements(values map[string]string) bool {
	if len(values) < 1 || len(values) > 10 {
		return false
	}
	for accountID, token := range values {
		if len(accountID) < 1 || len(accountID) > 128 || len(token) < 1 || len(token) > 2048 {
			return false
		}
	}
	return true
}

func writeSessionError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, db.ErrSessionNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return
	}
	httptransport.WriteError(writer, request, errors.New("session store unavailable"))
}

func writeSessionJSON(writer http.ResponseWriter, status int, value sessionResponse) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
