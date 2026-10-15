package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxCorrectionBodyBytes = 64 * 1024

type CorrectionHandler struct {
	service           *appcorrections.Service
	identity          identity.Provider
	authenticator     *identity.RequestAuthenticator
	rateLimiter       RateLimiter
	readRate          int
	writeRate         int
	capacityLimit     int
	audit             AuditRecorder
	committedObserver httptransport.CommittedResponseObserver
}

func NewCorrectionHandler(service *appcorrections.Service, provider identity.Provider) *CorrectionHandler {
	return &CorrectionHandler{service: service, identity: provider}
}

func (h *CorrectionHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *CorrectionHandler {
	h.authenticator = authenticator
	return h
}

func (h *CorrectionHandler) WithRateLimiter(limiter RateLimiter, readPerMinute, writePerMinute, capacityPerSecond int) *CorrectionHandler {
	h.rateLimiter, h.readRate, h.writeRate, h.capacityLimit = limiter, readPerMinute, writePerMinute, capacityPerSecond
	return h
}

func (h *CorrectionHandler) WithAuditRecorder(audit AuditRecorder) *CorrectionHandler {
	h.audit = audit
	return h
}

func (h *CorrectionHandler) WithCommittedResponseObserver(observer httptransport.CommittedResponseObserver) *CorrectionHandler {
	h.committedObserver = observer
	return h
}

type correctionRequest struct {
	ReasonCode   string `json:"reason_code"`
	OperatorNote string `json:"operator_note"`
}

type correctionDecisionRequest struct {
	Reason string `json:"reason"`
}

func (h *CorrectionHandler) Request(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "corrections:write", "corrections:request", true)
	if !ok {
		return
	}
	var input correctionRequest
	if decodeCorrectionJSON(writer, request, &input) != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Request(request.Context(), appcorrections.RequestCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		OriginalTransferID: request.PathValue("transferID"), ReasonCode: input.ReasonCode, OperatorNote: input.OperatorNote,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), CorrelationID: middleware.CorrelationID(request.Context()),
		StepUpAuthenticatedAt: principal.AuthenticatedAt,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicCorrectionError(err))
		return
	}
	writeCorrectionSubmission(request.Context(), writer, submission, http.StatusCreated, h.committedObserver)
}

func (h *CorrectionHandler) List(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "corrections:read", "corrections:list", false)
	if !ok {
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
	page, err := h.service.List(request.Context(), principal.TenantID, principal.SubjectID, appcorrections.Query{
		Status: request.URL.Query().Get("status"), Cursor: request.URL.Query().Get("cursor"), Limit: limit,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicCorrectionError(err))
		return
	}
	writeCorrectionJSON(writer, http.StatusOK, page)
}

func (h *CorrectionHandler) Get(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "corrections:read", "corrections:get", false)
	if !ok {
		return
	}
	event, err := h.service.Get(request.Context(), principal.TenantID, principal.SubjectID, request.PathValue("correctionId"))
	if err != nil {
		httptransport.WriteError(writer, request, publicCorrectionError(err))
		return
	}
	writeCorrectionJSON(writer, http.StatusOK, event)
}

func (h *CorrectionHandler) Approve(writer http.ResponseWriter, request *http.Request) {
	h.decide(writer, request, true)
}

func (h *CorrectionHandler) Reject(writer http.ResponseWriter, request *http.Request) {
	h.decide(writer, request, false)
}

func (h *CorrectionHandler) decide(writer http.ResponseWriter, request *http.Request, approve bool) {
	operation := "corrections:reject"
	if approve {
		operation = "corrections:approve"
	}
	principal, ok := h.authorize(writer, request, "corrections:approve", operation, true)
	if !ok {
		return
	}
	var input correctionDecisionRequest
	if decodeCorrectionJSON(writer, request, &input) != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	command := appcorrections.DecisionCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrectionID: request.PathValue("correctionId"),
		Reason: input.Reason, CorrelationID: middleware.CorrelationID(request.Context()), StepUpAuthenticatedAt: principal.AuthenticatedAt,
	}
	var event appcorrections.Event
	var err error
	if approve {
		event, err = h.service.Approve(request.Context(), command)
	} else {
		event, err = h.service.Reject(request.Context(), command)
	}
	if err != nil {
		httptransport.WriteError(writer, request, publicCorrectionError(err))
		return
	}
	writeCorrectionCommitted(request.Context(), writer, http.StatusOK, event, event.CorrectionID, false, h.committedObserver)
}

func (h *CorrectionHandler) Cancel(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "corrections:write", "corrections:cancel", true)
	if !ok {
		return
	}
	var input correctionDecisionRequest
	if decodeCorrectionJSON(writer, request, &input) != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	event, err := h.service.Cancel(request.Context(), appcorrections.CancelCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrectionID: request.PathValue("correctionId"),
		Reason: input.Reason, CorrelationID: middleware.CorrelationID(request.Context()),
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicCorrectionError(err))
		return
	}
	writeCorrectionCommitted(request.Context(), writer, http.StatusOK, event, event.CorrectionID, false, h.committedObserver)
}

func (h *CorrectionHandler) Post(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "corrections:approve", "corrections:post", true)
	if !ok {
		return
	}
	submission, err := h.service.Post(request.Context(), appcorrections.PostCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrectionID: request.PathValue("correctionId"),
		IdempotencyKey: request.Header.Get("Idempotency-Key"), CorrelationID: middleware.CorrelationID(request.Context()), StepUpAuthenticatedAt: principal.AuthenticatedAt,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicCorrectionError(err))
		return
	}
	writeCorrectionSubmission(request.Context(), writer, submission, http.StatusOK, h.committedObserver)
}

func (h *CorrectionHandler) authorize(writer http.ResponseWriter, request *http.Request, scope, operation string, write bool) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("correction handler is not configured"))
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

func (h *CorrectionHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func decodeCorrectionJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, maxCorrectionBodyBytes)
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

func writeCorrectionSubmission(ctx context.Context, writer http.ResponseWriter, submission appcorrections.Submission, status int, observer httptransport.CommittedResponseObserver) {
	writeCorrectionCommitted(ctx, writer, status, submission, submission.Event.CorrectionID, submission.Replayed, observer)
}

func writeCorrectionCommitted(ctx context.Context, writer http.ResponseWriter, status int, body any, commandID string, replayed bool, observer httptransport.CommittedResponseObserver) {
	headers := make(http.Header)
	if replayed {
		headers.Set("Idempotent-Replay", "true")
	}
	httptransport.WriteCommittedJSON(ctx, writer, httptransport.CommittedResponse{
		Status:       status,
		CommandKind:  "correction",
		CommandID:    commandID,
		RecoveryPath: "/api/transfer-corrections/" + commandID,
		Body:         body,
		Headers:      headers,
	}, observer)
}

func writeCorrectionJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func publicCorrectionError(err error) error {
	switch {
	case errors.Is(err, appcorrections.ErrInvalidCommand):
		return &httptransport.PublicError{Status: http.StatusBadRequest, Code: "invalid_correction_request", Message: "The correction request is invalid."}
	case errors.Is(err, appcorrections.ErrNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, appcorrections.ErrForbidden):
		return httptransport.ErrForbidden
	case errors.Is(err, appcorrections.ErrStepUpRequired):
		return &httptransport.PublicError{Status: http.StatusPreconditionRequired, Code: "step_up_required", Message: "Recent authentication is required for this correction command."}
	case errors.Is(err, appcorrections.ErrExpired):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "correction_expired", Message: "The correction approval window has expired."}
	case errors.Is(err, appcorrections.ErrConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "correction_conflict", Message: "The correction evidence changed or is no longer actionable."}
	default:
		return err
	}
}
