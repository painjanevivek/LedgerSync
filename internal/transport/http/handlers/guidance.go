package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/guidance"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

const (
	orientationDeadline    = 3 * time.Second
	preferenceDeadline     = 3 * time.Second
	explainabilityDeadline = 5 * time.Second
	maxPreferenceBodyBytes = 4 << 10
)

type orientationPreferenceRequest struct {
	ExpectedVersion  string   `json:"expected_version"`
	Dismissed        bool     `json:"dismissed"`
	CompletedStepIDs []string `json:"completed_step_ids"`
}

type GuidanceHandler struct {
	service       *guidance.Service
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
}

func NewGuidanceHandler(service *guidance.Service, provider identity.Provider) *GuidanceHandler {
	return &GuidanceHandler{service: service, identity: provider}
}

func (h *GuidanceHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *GuidanceHandler {
	h.authenticator = authenticator
	return h
}

func (h *GuidanceHandler) WithRateLimiter(limiter RateLimiter, limit int) *GuidanceHandler {
	h.rateLimiter, h.rateLimit = limiter, limit
	return h
}

func (h *GuidanceHandler) WithAuditRecorder(audit AuditRecorder) *GuidanceHandler {
	h.audit = audit
	return h
}

func (h *GuidanceHandler) Orientation(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, http.MethodGet, []string{"local:read"}, true, "local:orientation")
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), orientationDeadline)
	defer cancel()
	summary, err := h.service.Orientation(ctx, principal.TenantID, principal.SubjectID)
	if err != nil {
		writeGuidanceUnavailable(writer, request)
		return
	}
	writeGuidanceJSON(writer, summary)
}

func (h *GuidanceHandler) UpdateOrientationPreferences(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, http.MethodPut, []string{"local:write"}, true, "local:orientation-preferences")
	if !ok {
		return
	}
	input, err := decodeOrientationPreferenceRequest(writer, request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), preferenceDeadline)
	defer cancel()
	summary, err := h.service.UpdateOrientationPreferences(ctx, principal.TenantID, principal.SubjectID, input.ExpectedVersion, input.Dismissed, input.CompletedStepIDs)
	if errors.Is(err, guidance.ErrPreferenceConflict) {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusConflict, Code: "preference_version_conflict", Message: "The onboarding preference changed in another session. Refresh and try again."})
		return
	}
	if errors.Is(err, guidance.ErrInvalidPreference) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	if err != nil {
		writeGuidanceUnavailable(writer, request)
		return
	}
	writeGuidanceJSON(writer, summary)
}

func (h *GuidanceHandler) ExplainTransfer(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, http.MethodGet, []string{"explainability:read", "transfers:read", "events:read", "reconciliation:read"}, false, "transfers:explainability")
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), explainabilityDeadline)
	defer cancel()
	timeline, err := h.service.ExplainTransfer(ctx, principal.TenantID, principal.SubjectID, strings.TrimSpace(request.PathValue("transferID")))
	if errors.Is(err, guidance.ErrTransferNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		writeGuidanceUnavailable(writer, request)
		return
	}
	writeGuidanceJSON(writer, timeline)
}

func (h *GuidanceHandler) authorize(writer http.ResponseWriter, request *http.Request, method string, scopes []string, requireOperator bool, route string) (identity.Principal, bool) {
	if request.Method != method {
		writer.Header().Set("Allow", method)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: fmt.Sprintf("Only %s is allowed.", method)})
		return identity.Principal{}, false
	}
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("guidance handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, false
	}
	for _, scope := range scopes {
		if identity.RequireScope(principal, scope) != nil {
			writeScopeDenial(writer, request, h.audit, principal, scope)
			return identity.Principal{}, false
		}
	}
	if requireOperator && !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:orientation")
		return identity.Principal{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, h.rateLimit, false) {
		return identity.Principal{}, false
	}
	return principal, true
}

func decodeOrientationPreferenceRequest(writer http.ResponseWriter, request *http.Request) (orientationPreferenceRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxPreferenceBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input orientationPreferenceRequest
	if err := decoder.Decode(&input); err != nil {
		return orientationPreferenceRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return orientationPreferenceRequest{}, errors.New("request contains more than one JSON value")
	}
	return input, nil
}

func (h *GuidanceHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func writeGuidanceJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(writer).Encode(value)
}

func writeGuidanceUnavailable(writer http.ResponseWriter, request *http.Request) {
	httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Authoritative guided evidence is temporarily unavailable."})
}
