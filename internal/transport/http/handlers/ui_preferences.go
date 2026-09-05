package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

type UIPreferenceHandler struct {
	repository    uiPreferenceStore
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
}

type uiPreferenceStore interface {
	Get(context.Context, string, string) (db.UIPreference, error)
	Update(context.Context, string, string, string, int64) (db.UIPreference, error)
}

type uiPreferenceRequest struct {
	ExperienceMode  string `json:"experience_mode"`
	ExpectedVersion string `json:"expected_version"`
}

type uiPreferenceResponse struct {
	ExperienceMode string `json:"experience_mode"`
	Version        string `json:"version"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

func NewUIPreferenceHandler(repository uiPreferenceStore, provider identity.Provider, authenticator *identity.RequestAuthenticator) *UIPreferenceHandler {
	return &UIPreferenceHandler{repository: repository, identity: provider, authenticator: authenticator}
}

func (h *UIPreferenceHandler) Get(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	preference, err := h.repository.Get(ctx, principal.TenantID, principal.SubjectID)
	if err != nil {
		httptransport.WriteError(writer, request, errors.New("UI preference unavailable"))
		return
	}
	writeUIPreferenceJSON(writer, preference)
}

func (h *UIPreferenceHandler) Patch(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request)
	if !ok {
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1024)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input uiPreferenceRequest
	if request.Header.Get("Content-Type") != "application/json" || decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || (input.ExperienceMode != "simple" && input.ExperienceMode != "expert") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	version, err := strconv.ParseInt(input.ExpectedVersion, 10, 64)
	if err != nil || version < 0 {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	preference, err := h.repository.Update(ctx, principal.TenantID, principal.SubjectID, input.ExperienceMode, version)
	if errors.Is(err, db.ErrUIPreferenceConflict) {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusConflict, Code: "preference_version_conflict", Message: "The interface preference changed in another session. Refresh and try again."})
		return
	}
	if err != nil {
		httptransport.WriteError(writer, request, errors.New("UI preference unavailable"))
		return
	}
	writeUIPreferenceJSON(writer, preference)
}

func (h *UIPreferenceHandler) authorize(writer http.ResponseWriter, request *http.Request) (identity.Principal, bool) {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("UI preference handler is not configured"))
		return identity.Principal{}, false
	}
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	var principal identity.Principal
	var err error
	if h.authenticator != nil {
		principal, err = h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	} else {
		principal, err = h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
	}
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, nil, principal, "tenant:ui-preferences")
		return identity.Principal{}, false
	}
	return principal, true
}

func writeUIPreferenceJSON(writer http.ResponseWriter, preference db.UIPreference) {
	response := uiPreferenceResponse{ExperienceMode: preference.ExperienceMode, Version: strconv.FormatInt(preference.Version, 10)}
	if preference.UpdatedAt != nil {
		response.UpdatedAt = preference.UpdatedAt.Format(time.RFC3339Nano)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(response)
}
