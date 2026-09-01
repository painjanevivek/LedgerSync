package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type DeveloperWebhookHandler struct {
	service                            *developerplatform.WebhookService
	identity                           identity.Provider
	authenticator                      *identity.RequestAuthenticator
	rateLimiter                        RateLimiter
	readRate, writeRate, capacityLimit int
	audit                              AuditRecorder
	requireStepUp                      bool
	clock                              func() time.Time
}

func NewDeveloperWebhookHandler(service *developerplatform.WebhookService, provider identity.Provider) *DeveloperWebhookHandler {
	return &DeveloperWebhookHandler{service: service, identity: provider, clock: time.Now}
}
func (h *DeveloperWebhookHandler) WithRequestAuthenticator(v *identity.RequestAuthenticator) *DeveloperWebhookHandler {
	h.authenticator = v
	return h
}
func (h *DeveloperWebhookHandler) WithRateLimiter(v RateLimiter, readRate, writeRate, capacity int) *DeveloperWebhookHandler {
	h.rateLimiter, h.readRate, h.writeRate, h.capacityLimit = v, readRate, writeRate, capacity
	return h
}
func (h *DeveloperWebhookHandler) WithAuditRecorder(v AuditRecorder) *DeveloperWebhookHandler {
	h.audit = v
	return h
}
func (h *DeveloperWebhookHandler) WithProductionStepUp(v bool) *DeveloperWebhookHandler {
	h.requireStepUp = v
	return h
}

type webhookRegisterRequest struct {
	DisplayName         string   `json:"display_name"`
	EndpointURL         string   `json:"endpoint_url"`
	SubscribedEvents    []string `json:"subscribed_events"`
	SigningKeyReference string   `json:"signing_key_reference"`
	SigningKeyID        string   `json:"signing_key_id"`
}
type webhookRotateRequest struct {
	ExpectedVersion     string `json:"expected_version"`
	SigningKeyReference string `json:"signing_key_reference"`
	SigningKeyID        string `json:"signing_key_id"`
}
type webhookDisableRequest struct {
	ExpectedVersion string `json:"expected_version"`
	Reason          string `json:"reason"`
}
type webhookReplayApprovalRequest struct {
	ReasonCode string `json:"reason_code"`
}
type webhookReplayRequest struct {
	ApprovalID string `json:"approval_id"`
}

func (h *DeveloperWebhookHandler) Register(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:write", "webhooks:register", true)
	if !ok {
		return
	}
	var input webhookRegisterRequest
	if decodeDeveloperCredentialJSON(w, r, &input) != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Register(r.Context(), developerplatform.RegisterWebhookCommand{TenantID: p.TenantID, ActorSubjectID: p.SubjectID, CorrelationID: middleware.CorrelationID(r.Context()), IdempotencyKey: r.Header.Get("Idempotency-Key"), DisplayName: input.DisplayName, EndpointURL: input.EndpointURL, SubscribedEvents: input.SubscribedEvents, SigningKeyReference: input.SigningKeyReference, SigningKeyID: input.SigningKeyID})
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	if submission.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeDeveloperCredentialJSON(w, http.StatusCreated, submission.Webhook)
}
func (h *DeveloperWebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:read", "webhooks:list", false)
	if !ok {
		return
	}
	if !onlyQueryKeys(r, "status", "cursor", "limit") {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	limit, ok := webhookLimit(w, r)
	if !ok {
		return
	}
	page, err := h.service.List(r.Context(), p.TenantID, developerplatform.WebhookQuery{Status: r.URL.Query().Get("status"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	writeDeveloperCredentialJSON(w, http.StatusOK, page)
}
func (h *DeveloperWebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:read", "webhooks:get", false)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	item, err := h.service.Get(r.Context(), p.TenantID, r.PathValue("webhookId"))
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	writeDeveloperCredentialJSON(w, http.StatusOK, item)
}
func (h *DeveloperWebhookHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:write", "webhooks:rotate", true)
	if !ok {
		return
	}
	var input webhookRotateRequest
	if decodeDeveloperCredentialJSON(w, r, &input) != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	version, err := strconv.ParseInt(input.ExpectedVersion, 10, 64)
	if err != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Rotate(r.Context(), developerplatform.RotateWebhookCommand{TenantID: p.TenantID, ActorSubjectID: p.SubjectID, CorrelationID: middleware.CorrelationID(r.Context()), IdempotencyKey: r.Header.Get("Idempotency-Key"), WebhookID: r.PathValue("webhookId"), ExpectedVersion: version, SigningKeyReference: input.SigningKeyReference, SigningKeyID: input.SigningKeyID})
	h.writeSubmission(w, r, submission, err)
}
func (h *DeveloperWebhookHandler) Disable(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:write", "webhooks:disable", true)
	if !ok {
		return
	}
	var input webhookDisableRequest
	if decodeDeveloperCredentialJSON(w, r, &input) != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	version, err := strconv.ParseInt(input.ExpectedVersion, 10, 64)
	if err != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Disable(r.Context(), developerplatform.DisableWebhookCommand{TenantID: p.TenantID, ActorSubjectID: p.SubjectID, CorrelationID: middleware.CorrelationID(r.Context()), IdempotencyKey: r.Header.Get("Idempotency-Key"), WebhookID: r.PathValue("webhookId"), ExpectedVersion: version, Reason: input.Reason})
	h.writeSubmission(w, r, submission, err)
}
func (h *DeveloperWebhookHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:read", "webhooks:deliveries", false)
	if !ok {
		return
	}
	if !onlyQueryKeys(r, "status", "cursor", "limit") {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	limit, ok := webhookLimit(w, r)
	if !ok {
		return
	}
	page, err := h.service.Deliveries(r.Context(), p.TenantID, r.PathValue("webhookId"), developerplatform.DeliveryQuery{Status: r.URL.Query().Get("status"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	writeDeveloperCredentialJSON(w, http.StatusOK, page)
}
func (h *DeveloperWebhookHandler) ApproveReplay(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:replay", "webhooks:replay_approve", true)
	if !ok {
		return
	}
	var input webhookReplayApprovalRequest
	if decodeDeveloperCredentialJSON(w, r, &input) != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	result, err := h.service.ApproveReplay(r.Context(), p.TenantID, r.PathValue("webhookId"), r.PathValue("attemptId"), p.SubjectID, input.ReasonCode, middleware.CorrelationID(r.Context()), r.Header.Get("Idempotency-Key"))
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	if result.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeDeveloperCredentialJSON(w, http.StatusCreated, map[string]string{"approval_id": result.ApprovalID, "status": "approved"})
}
func (h *DeveloperWebhookHandler) Replay(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, "webhooks:replay", "webhooks:replay_execute", true)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || r.Body == nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	var input webhookReplayRequest
	if decodeDeveloperCredentialJSON(w, r, &input) != nil {
		httptransport.WriteError(w, r, httptransport.ErrBadRequest)
		return
	}
	result, err := h.service.ReplayDelivery(r.Context(), p.TenantID, r.PathValue("webhookId"), r.PathValue("attemptId"), input.ApprovalID, p.SubjectID, middleware.CorrelationID(r.Context()), r.Header.Get("Idempotency-Key"))
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	if result.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeDeveloperCredentialJSON(w, http.StatusAccepted, map[string]string{"delivery_job_id": result.DeliveryJobID, "status": "scheduled"})
}

func (h *DeveloperWebhookHandler) writeSubmission(w http.ResponseWriter, r *http.Request, s developerplatform.WebhookSubmission, err error) {
	if err != nil {
		httptransport.WriteError(w, r, publicDeveloperWebhookError(err))
		return
	}
	if s.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeDeveloperCredentialJSON(w, http.StatusOK, s.Webhook)
}
func (h *DeveloperWebhookHandler) authorize(w http.ResponseWriter, r *http.Request, scope, operation string, write bool) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(w, r, errors.New("developer webhook handler is not configured"))
		return identity.Principal{}, false
	}
	p, err := h.authenticate(r)
	if err != nil {
		writeAuthenticationError(w, r, err)
		return identity.Principal{}, false
	}
	if identity.RequireScope(p, scope) != nil {
		writeScopeDenial(w, r, h.audit, p, scope)
		return identity.Principal{}, false
	}
	if write && h.requireStepUp && (p.AuthenticatedAt.IsZero() || h.clock().UTC().Sub(p.AuthenticatedAt) > 10*time.Minute || p.AuthenticatedAt.After(h.clock().UTC().Add(time.Minute))) {
		httptransport.WriteError(w, r, &httptransport.PublicError{Status: http.StatusForbidden, Code: "step_up_required", Message: "Recent operator authentication is required."})
		return identity.Principal{}, false
	}
	if write && !enforceTenantCapacity(w, r, h.rateLimiter, p, operation, h.capacityLimit) {
		return identity.Principal{}, false
	}
	rate := h.readRate
	if write {
		rate = h.writeRate
	}
	if !enforceRateLimit(w, r, h.rateLimiter, p, operation, rate, write) {
		return identity.Principal{}, false
	}
	return p, true
}
func (h *DeveloperWebhookHandler) authenticate(r *http.Request) (identity.Principal, error) {
	assertion := r.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(r.Context(), bearerToken(r.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(r.Context(), bearerToken(r.Header.Get("Authorization")))
}
func webhookLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			httptransport.WriteError(w, r, httptransport.ErrBadRequest)
			return 0, false
		}
		limit = value
	}
	return limit, true
}
func publicDeveloperWebhookError(err error) error {
	switch {
	case errors.Is(err, developerplatform.ErrInvalidCommand):
		return &httptransport.PublicError{Status: http.StatusBadRequest, Code: "invalid_webhook_request", Message: "The webhook request is invalid."}
	case errors.Is(err, developerplatform.ErrNotFound), errors.Is(err, db.ErrDeadDeliveryNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, developerplatform.ErrVersionConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "webhook_version_conflict", Message: "The webhook changed before this command was applied."}
	case errors.Is(err, developerplatform.ErrIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "The idempotency key belongs to a different webhook intent."}
	case errors.Is(err, db.ErrDeliveryReplayIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "The idempotency key belongs to a different delivery replay intent."}
	case errors.Is(err, db.ErrDeliveryReplayNotApproved):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "replay_not_approved", Message: "This delivery replay does not have the referenced approval."}
	case errors.Is(err, db.ErrReplaySeparationRequired):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "replay_separation_required", Message: "The replay operator must differ from the approver."}
	case errors.Is(err, developerplatform.ErrConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "webhook_conflict", Message: "The webhook is no longer actionable."}
	default:
		return err
	}
}
