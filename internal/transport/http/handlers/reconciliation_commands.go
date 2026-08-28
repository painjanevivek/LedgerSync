package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxReconciliationCommandBodyBytes = 1024

var errUnsupportedReconciliationCommandMediaType = errors.New("reconciliation command content type must be application/json")

type ReconciliationCommandHandler struct {
	service       *reconciliation.CommandService
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	capacityLimit int
	audit         AuditRecorder
}

func NewReconciliationCommandHandler(service *reconciliation.CommandService, provider identity.Provider) *ReconciliationCommandHandler {
	return &ReconciliationCommandHandler{service: service, identity: provider}
}

func (h *ReconciliationCommandHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *ReconciliationCommandHandler {
	h.authenticator = authenticator
	return h
}
func (h *ReconciliationCommandHandler) WithRateLimiter(limiter RateLimiter, limit int) *ReconciliationCommandHandler {
	h.rateLimiter, h.rateLimit = limiter, limit
	return h
}
func (h *ReconciliationCommandHandler) WithCapacityLimit(limiter RateLimiter, limit int) *ReconciliationCommandHandler {
	h.rateLimiter, h.capacityLimit = limiter, limit
	return h
}
func (h *ReconciliationCommandHandler) WithAuditRecorder(audit AuditRecorder) *ReconciliationCommandHandler {
	h.audit = audit
	return h
}

func (h *ReconciliationCommandHandler) Run(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only POST is allowed."})
		return
	}
	principal, ok := h.authorize(writer, request)
	if !ok {
		return
	}
	if err := decodeReconciliationCommandRequest(writer, request); err != nil {
		if errors.Is(err, errUnsupportedReconciliationCommandMediaType) {
			httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type must be application/json."})
		} else {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		}
		return
	}
	submission, err := h.service.Run(request.Context(), reconciliation.RunCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicReconciliationCommandError(err))
		return
	}
	if submission.Denial == "already_running" {
		writeReconciliationAlreadyRunning(writer, request, submission)
		return
	}
	if submission.Denial == "response_unknown" {
		if submission.Replayed {
			writer.Header().Set("Idempotent-Replay", "true")
		}
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusGatewayTimeout, Code: "response_unknown", Message: "The reconciliation outcome is unknown. Start a new run with a new idempotency key."})
		return
	}
	writeReconciliationCommandResult(writer, submission)
}

func decodeReconciliationCommandRequest(writer http.ResponseWriter, request *http.Request) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errUnsupportedReconciliationCommandMediaType
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxReconciliationCommandBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input struct{}
	if err := decoder.Decode(&input); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains more than one JSON value")
	}
	return nil
}

func writeReconciliationAlreadyRunning(writer http.ResponseWriter, request *http.Request, submission reconciliation.CommandSubmission) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	if submission.Replayed {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writer.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(writer).Encode(reconciliationCommandErrorEnvelope{
		Error: reconciliationCommandError{Code: "reconciliation_already_running", Message: "A reconciliation run is already active for this tenant.", RequestID: middleware.CorrelationID(request.Context())},
		RunID: submission.ActiveRunID,
	})
}

type reconciliationCommandError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type reconciliationCommandErrorEnvelope struct {
	Error reconciliationCommandError `json:"error"`
	RunID string                     `json:"run_id,omitempty"`
}

func (h *ReconciliationCommandHandler) authorize(writer http.ResponseWriter, request *http.Request) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("reconciliation command handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return identity.Principal{}, false
	}
	if identity.RequireScope(principal, "reconciliation:write") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "reconciliation:write")
		return identity.Principal{}, false
	}
	if !enforceTenantCapacity(writer, request, h.rateLimiter, principal, "reconciliation:run", h.capacityLimit) || !enforceRateLimit(writer, request, h.rateLimiter, principal, "reconciliation:run", h.rateLimit, true) {
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *ReconciliationCommandHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

type reconciliationCommandResponse struct {
	RunID               string `json:"run_id"`
	Status              string `json:"status"`
	CorrelationID       string `json:"correlation_id"`
	Scope               string `json:"scope"`
	LedgerWatermark     string `json:"ledger_watermark"`
	ApplicationVersion  string `json:"application_version"`
	SchemaVersion       string `json:"schema_version"`
	CheckedAccountCount string `json:"checked_account_count"`
	PostingCount        string `json:"posting_count"`
	MismatchCount       string `json:"mismatch_count"`
	StartedAt           string `json:"started_at"`
	CompletedAt         string `json:"completed_at"`
}

func writeReconciliationCommandResult(writer http.ResponseWriter, submission reconciliation.CommandSubmission) {
	result := submission.Result
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	if submission.Replayed {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(reconciliationCommandResponse{
		RunID: result.ID, Status: string(result.Status), CorrelationID: result.CorrelationID, Scope: result.Scope,
		LedgerWatermark: result.LedgerWatermark, ApplicationVersion: result.ApplicationVersion, SchemaVersion: result.SchemaVersion,
		CheckedAccountCount: strconv.Itoa(result.CheckedAccountCount), PostingCount: strconv.Itoa(result.PostingCount), MismatchCount: strconv.Itoa(result.MismatchCount),
		StartedAt: result.StartedAt.UTC().Format("2006-01-02T15:04:05.000000Z"), CompletedAt: result.CompletedAt.UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
}

func publicReconciliationCommandError(err error) error {
	switch {
	case errors.Is(err, reconciliation.ErrInvalidCommand), errors.Is(err, idempotency.ErrInvalidKey):
		return httptransport.ErrBadRequest
	case errors.Is(err, reconciliation.ErrIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "This idempotency key belongs to a different request."}
	case errors.Is(err, reconciliation.ErrCommandInProgress):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "request_in_progress", Message: "A matching request is still being completed. Retry with the same idempotency key."}
	case errors.Is(err, reconciliation.ErrResponseUnknown), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return &httptransport.PublicError{Status: http.StatusGatewayTimeout, Code: "response_unknown", Message: "The reconciliation outcome is unknown. Retry with the identical idempotency key."}
	case errors.Is(err, reconciliation.ErrCommandUnavailable), db.IsRetryableTransactionError(err):
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "temporary_unavailable", Message: "Reconciliation is temporarily unavailable. Retry with the identical idempotency key."}
	default:
		return err
	}
}
