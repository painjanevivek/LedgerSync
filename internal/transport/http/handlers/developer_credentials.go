package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxDeveloperCredentialBody = 16 * 1024

type DeveloperCredentialHandler struct {
	service       *developerplatform.CredentialService
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	readRate      int
	writeRate     int
	capacityLimit int
	audit         AuditRecorder
	requireStepUp bool
	clock         func() time.Time
}

func NewDeveloperCredentialHandler(service *developerplatform.CredentialService, provider identity.Provider) *DeveloperCredentialHandler {
	return &DeveloperCredentialHandler{service: service, identity: provider, clock: time.Now}
}

func (h *DeveloperCredentialHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *DeveloperCredentialHandler {
	h.authenticator = authenticator
	return h
}

func (h *DeveloperCredentialHandler) WithRateLimiter(limiter RateLimiter, readRate, writeRate, capacity int) *DeveloperCredentialHandler {
	h.rateLimiter, h.readRate, h.writeRate, h.capacityLimit = limiter, readRate, writeRate, capacity
	return h
}

func (h *DeveloperCredentialHandler) WithAuditRecorder(audit AuditRecorder) *DeveloperCredentialHandler {
	h.audit = audit
	return h
}

func (h *DeveloperCredentialHandler) WithProductionStepUp(required bool) *DeveloperCredentialHandler {
	h.requireStepUp = required
	return h
}

type developerCredentialCreateRequest struct {
	DisplayName       string   `json:"display_name"`
	ExternalReference string   `json:"external_reference"`
	Audience          string   `json:"audience"`
	Scopes            []string `json:"scopes"`
	ExpiresAt         string   `json:"expires_at"`
}

type developerCredentialRotateRequest struct {
	ExpectedVersion   string   `json:"expected_version"`
	ExternalReference string   `json:"external_reference"`
	Audience          string   `json:"audience"`
	Scopes            []string `json:"scopes"`
	ExpiresAt         string   `json:"expires_at"`
}

type developerCredentialRevokeRequest struct {
	ExpectedVersion string `json:"expected_version"`
	Reason          string `json:"reason"`
}

func (h *DeveloperCredentialHandler) Create(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "credentials:write", "credentials:create", true)
	if !ok {
		return
	}
	var input developerCredentialCreateRequest
	if err := decodeDeveloperCredentialJSON(writer, request, &input); err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, input.ExpiresAt)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Create(request.Context(), developerplatform.CreateCredentialCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"),
		DisplayName: input.DisplayName, ExternalReference: input.ExternalReference, Audience: input.Audience, Scopes: input.Scopes, ExpiresAt: expiresAt,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicDeveloperCredentialError(err))
		return
	}
	writeDeveloperCredentialSubmission(writer, http.StatusCreated, submission)
}

func (h *DeveloperCredentialHandler) List(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "credentials:read", "credentials:list", false)
	if !ok {
		return
	}
	if !onlyQueryKeys(request, "status", "cursor", "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	limit := 50
	var err error
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
			return
		}
	}
	page, err := h.service.List(request.Context(), principal.TenantID, developerplatform.CredentialQuery{Status: request.URL.Query().Get("status"), Cursor: request.URL.Query().Get("cursor"), Limit: limit})
	if err != nil {
		httptransport.WriteError(writer, request, publicDeveloperCredentialError(err))
		return
	}
	writeDeveloperCredentialJSON(writer, http.StatusOK, page)
}

func (h *DeveloperCredentialHandler) Get(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "credentials:read", "credentials:get", false)
	if !ok {
		return
	}
	if request.URL.RawQuery != "" {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	credential, err := h.service.Get(request.Context(), principal.TenantID, request.PathValue("credentialId"))
	if err != nil {
		httptransport.WriteError(writer, request, publicDeveloperCredentialError(err))
		return
	}
	writeDeveloperCredentialJSON(writer, http.StatusOK, credential)
}

func (h *DeveloperCredentialHandler) Rotate(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "credentials:write", "credentials:rotate", true)
	if !ok {
		return
	}
	var input developerCredentialRotateRequest
	if err := decodeDeveloperCredentialJSON(writer, request, &input); err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	version, versionErr := strconv.ParseInt(input.ExpectedVersion, 10, 64)
	expiresAt, expiryErr := time.Parse(time.RFC3339, input.ExpiresAt)
	if versionErr != nil || expiryErr != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Rotate(request.Context(), developerplatform.RotateCredentialCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"), CredentialID: request.PathValue("credentialId"),
		ExpectedVersion: version, ExternalReference: input.ExternalReference, Audience: input.Audience, Scopes: input.Scopes, ExpiresAt: expiresAt,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicDeveloperCredentialError(err))
		return
	}
	writeDeveloperCredentialSubmission(writer, http.StatusOK, submission)
}

func (h *DeveloperCredentialHandler) Revoke(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "credentials:write", "credentials:revoke", true)
	if !ok {
		return
	}
	var input developerCredentialRevokeRequest
	if err := decodeDeveloperCredentialJSON(writer, request, &input); err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	version, err := strconv.ParseInt(input.ExpectedVersion, 10, 64)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Revoke(request.Context(), developerplatform.RevokeCredentialCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"), CredentialID: request.PathValue("credentialId"), ExpectedVersion: version, Reason: input.Reason,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicDeveloperCredentialError(err))
		return
	}
	writeDeveloperCredentialSubmission(writer, http.StatusOK, submission)
}

func (h *DeveloperCredentialHandler) authorize(writer http.ResponseWriter, request *http.Request, scope, operation string, write bool) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("developer credential handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, false
	}
	if identity.RequireScope(principal, scope) != nil {
		writeScopeDenial(writer, request, h.audit, principal, scope)
		return identity.Principal{}, false
	}
	if write && h.requireStepUp && (principal.AuthenticatedAt.IsZero() || h.clock().UTC().Sub(principal.AuthenticatedAt) > 10*time.Minute || principal.AuthenticatedAt.After(h.clock().UTC().Add(time.Minute))) {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusForbidden, Code: "step_up_required", Message: "Recent operator authentication is required."})
		return identity.Principal{}, false
	}
	if write && !enforceTenantCapacity(writer, request, h.rateLimiter, principal, operation, h.capacityLimit) {
		return identity.Principal{}, false
	}
	rate := h.readRate
	if write {
		rate = h.writeRate
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, operation, rate, write) {
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *DeveloperCredentialHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func decodeDeveloperCredentialJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	if request.URL.RawQuery != "" || !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/json") {
		return errors.New("invalid developer credential request")
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxDeveloperCredentialBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request contains more than one JSON value")
	}
	return nil
}

func onlyQueryKeys(request *http.Request, allowed ...string) bool {
	set := map[string]struct{}{}
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key, values := range request.URL.Query() {
		if _, ok := set[key]; !ok || len(values) != 1 {
			return false
		}
	}
	return true
}

func writeDeveloperCredentialSubmission(writer http.ResponseWriter, status int, submission developerplatform.CredentialSubmission) {
	if submission.Replayed {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeDeveloperCredentialJSON(writer, status, submission.Credential)
}

func writeDeveloperCredentialJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func publicDeveloperCredentialError(err error) error {
	switch {
	case errors.Is(err, developerplatform.ErrInvalidCommand):
		return &httptransport.PublicError{Status: http.StatusBadRequest, Code: "invalid_credential_request", Message: "The credential metadata request is invalid."}
	case errors.Is(err, developerplatform.ErrNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, developerplatform.ErrVersionConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "credential_version_conflict", Message: "The credential metadata changed before this command was applied."}
	case errors.Is(err, developerplatform.ErrIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "The idempotency key belongs to a different credential intent."}
	case errors.Is(err, developerplatform.ErrConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "credential_conflict", Message: "The credential is no longer actionable."}
	default:
		return err
	}
}
