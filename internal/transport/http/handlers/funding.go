package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxFundingBodyBytes = 64 * 1024

type FundingHandler struct {
	service       *appfunding.Service
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	readRate      int
	writeRate     int
	capacityLimit int
	audit         AuditRecorder
}

func NewFundingHandler(service *appfunding.Service, provider identity.Provider) *FundingHandler {
	return &FundingHandler{service: service, identity: provider}
}

func (h *FundingHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *FundingHandler {
	h.authenticator = authenticator
	return h
}

func (h *FundingHandler) WithRateLimiter(limiter RateLimiter, readPerMinute, writePerMinute, capacityPerSecond int) *FundingHandler {
	h.rateLimiter, h.readRate, h.writeRate, h.capacityLimit = limiter, readPerMinute, writePerMinute, capacityPerSecond
	return h
}

func (h *FundingHandler) WithAuditRecorder(audit AuditRecorder) *FundingHandler {
	h.audit = audit
	return h
}

type fundingRequest struct {
	DestinationAccountID string `json:"destination_account_id"`
	AmountMinor          string `json:"amount_minor"`
	Currency             string `json:"currency"`
	ExternalReference    string `json:"external_reference"`
	EvidenceReference    string `json:"evidence_reference"`
}

type fundingDecisionRequest struct {
	Reason string `json:"reason"`
}

type fundingCompensationRequest struct {
	ReasonCode   string `json:"reason_code"`
	OperatorNote string `json:"operator_note"`
}

func (h *FundingHandler) Request(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "funding:write", "funding:request", true)
	if !ok {
		return
	}
	var input fundingRequest
	if decodeErr := decodeFundingJSON(writer, request, &input); decodeErr != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	minor, err := appfunding.ParseMinor(input.AmountMinor)
	if err != nil || minor == 0 {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	amount, err := money.New(strings.TrimSpace(input.Currency), minor)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Request(request.Context(), appfunding.RequestCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		DestinationAccountID: input.DestinationAccountID, Amount: amount,
		ExternalReference: input.ExternalReference, EvidenceReference: input.EvidenceReference,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), CorrelationID: middleware.CorrelationID(request.Context()),
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingSubmission(writer, submission, http.StatusCreated)
}

func (h *FundingHandler) List(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "funding:read", "funding:list", false)
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
	page, err := h.service.List(request.Context(), principal.TenantID, principal.SubjectID, appfunding.Query{
		Status: request.URL.Query().Get("status"), Cursor: request.URL.Query().Get("cursor"), Limit: limit,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingJSON(writer, http.StatusOK, page)
}

func (h *FundingHandler) Get(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "funding:read", "funding:get", false)
	if !ok {
		return
	}
	event, err := h.service.Get(request.Context(), principal.TenantID, principal.SubjectID, request.PathValue("fundingEventId"))
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingJSON(writer, http.StatusOK, event)
}

func (h *FundingHandler) Approve(writer http.ResponseWriter, request *http.Request) {
	h.decide(writer, request, true)
}

func (h *FundingHandler) Reject(writer http.ResponseWriter, request *http.Request) {
	h.decide(writer, request, false)
}

func (h *FundingHandler) decide(writer http.ResponseWriter, request *http.Request, approve bool) {
	operation := "funding:reject"
	if approve {
		operation = "funding:approve"
	}
	principal, ok := h.authorize(writer, request, "funding:approve", operation, true)
	if !ok {
		return
	}
	var input fundingDecisionRequest
	if err := decodeFundingJSON(writer, request, &input); err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	command := appfunding.DecisionCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		FundingEventID: request.PathValue("fundingEventId"), Reason: input.Reason,
		CorrelationID: middleware.CorrelationID(request.Context()),
	}
	var event appfunding.Event
	var err error
	if approve {
		event, err = h.service.Approve(request.Context(), command)
	} else {
		event, err = h.service.Reject(request.Context(), command)
	}
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingJSON(writer, http.StatusOK, event)
}

func (h *FundingHandler) Post(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "funding:write", "funding:post", true)
	if !ok {
		return
	}
	submission, err := h.service.Post(request.Context(), appfunding.ActionCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		FundingEventID: request.PathValue("fundingEventId"), CorrelationID: middleware.CorrelationID(request.Context()),
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingSubmission(writer, submission, http.StatusOK)
}

func (h *FundingHandler) Compensate(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "funding:write", "funding:compensate", true)
	if !ok {
		return
	}
	var input fundingCompensationRequest
	if err := decodeFundingJSON(writer, request, &input); err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Compensate(request.Context(), appfunding.CompensationCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		FundingEventID: request.PathValue("fundingEventId"), ReasonCode: input.ReasonCode, OperatorNote: input.OperatorNote,
		IdempotencyKey: request.Header.Get("Idempotency-Key"), CorrelationID: middleware.CorrelationID(request.Context()),
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingSubmission(writer, submission, http.StatusCreated)
}

func (h *FundingHandler) Reconcile(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "funding:read", "funding:reconcile", false)
	if !ok {
		return
	}
	result, err := h.service.Reconcile(request.Context(), principal.TenantID, principal.SubjectID, request.PathValue("fundingEventId"))
	if err != nil {
		httptransport.WriteError(writer, request, publicFundingError(err))
		return
	}
	writeFundingJSON(writer, http.StatusOK, result)
}

func (h *FundingHandler) authorize(writer http.ResponseWriter, request *http.Request, scope, operation string, write bool) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("funding handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
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

func (h *FundingHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func decodeFundingJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, maxFundingBodyBytes)
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

func writeFundingSubmission(writer http.ResponseWriter, submission appfunding.Submission, status int) {
	if submission.Replayed {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeFundingJSON(writer, status, submission)
}

func writeFundingJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func publicFundingError(err error) error {
	switch {
	case errors.Is(err, appfunding.ErrInvalidCommand):
		return &httptransport.PublicError{Status: http.StatusBadRequest, Code: "invalid_funding_request", Message: "The funding request is invalid."}
	case errors.Is(err, appfunding.ErrNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, appfunding.ErrForbidden):
		return httptransport.ErrForbidden
	case errors.Is(err, appfunding.ErrConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "funding_conflict", Message: "The funding evidence changed or is no longer actionable."}
	case errors.Is(err, appfunding.ErrLimitExceeded):
		return &httptransport.PublicError{Status: http.StatusUnprocessableEntity, Code: "funding_limit_exceeded", Message: "The request is outside the approved funding limits."}
	default:
		return err
	}
}
