package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxAccountCommandBodyBytes = 4 * 1024

var errUnsupportedAccountCommandMediaType = errors.New("account command content type must be application/json")

type AccountCommandHandler struct {
	service       *accounts.CommandService
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	capacityLimit int
	audit         AuditRecorder
}

func NewAccountCommandHandler(service *accounts.CommandService, provider identity.Provider) *AccountCommandHandler {
	return &AccountCommandHandler{service: service, identity: provider}
}

func (h *AccountCommandHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *AccountCommandHandler {
	h.authenticator = authenticator
	return h
}

func (h *AccountCommandHandler) WithRateLimiter(limiter RateLimiter, requestsPerMinute int) *AccountCommandHandler {
	h.rateLimiter, h.rateLimit = limiter, requestsPerMinute
	return h
}

func (h *AccountCommandHandler) WithCapacityLimit(limiter RateLimiter, requestsPerSecond int) *AccountCommandHandler {
	h.rateLimiter, h.capacityLimit = limiter, requestsPerSecond
	return h
}

func (h *AccountCommandHandler) WithAuditRecorder(audit AuditRecorder) *AccountCommandHandler {
	h.audit = audit
	return h
}

type createAccountRequest struct {
	DisplayName       string `json:"display_name"`
	ExternalReference string `json:"external_reference"`
	Category          string `json:"category"`
	Currency          string `json:"currency"`
}

type patchAccountRequest struct {
	ExpectedVersion   string  `json:"expected_version"`
	DisplayName       *string `json:"display_name"`
	ExternalReference *string `json:"external_reference"`
	Category          *string `json:"category"`
	TargetStatus      *string `json:"target_status"`
	Reason            *string `json:"reason"`
}

func (h *AccountCommandHandler) Create(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only POST is allowed."})
		return
	}
	principal, ok := h.authorize(writer, request, "accounts:create")
	if !ok {
		return
	}
	var input createAccountRequest
	if err := decodeAccountCommandRequest(writer, request, &input); err != nil {
		writeAccountCommandDecodeError(writer, request, err)
		return
	}
	submission, err := h.service.Create(request.Context(), accounts.CreateAccountCommand{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID,
		CorrelationID: middleware.CorrelationID(request.Context()), IdempotencyKey: request.Header.Get("Idempotency-Key"),
		DisplayName: input.DisplayName, Reference: input.ExternalReference, Category: input.Category, Currency: input.Currency,
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicAccountCommandError(err))
		return
	}
	writeAccountCommandResult(writer, submission, http.StatusCreated)
}

func (h *AccountCommandHandler) Patch(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPatch {
		writer.Header().Set("Allow", http.MethodPatch)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only PATCH is allowed."})
		return
	}
	principal, ok := h.authorize(writer, request, "accounts:update")
	if !ok {
		return
	}
	accountID := strings.TrimSpace(request.PathValue("accountID"))
	if accountID == "" {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	var input patchAccountRequest
	if err := decodeAccountCommandRequest(writer, request, &input); err != nil {
		writeAccountCommandDecodeError(writer, request, err)
		return
	}
	expectedVersion, err := parseExpectedVersion(input.ExpectedVersion)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	metadataMutation := input.DisplayName != nil || input.ExternalReference != nil || input.Category != nil
	metadataComplete := input.DisplayName != nil && input.ExternalReference != nil && input.Category != nil
	statusMutation := input.TargetStatus != nil || input.Reason != nil
	statusComplete := input.TargetStatus != nil && input.Reason != nil
	if metadataMutation && (!metadataComplete || statusMutation) || statusMutation && (!statusComplete || metadataMutation) || !metadataMutation && !statusMutation {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	var submission accounts.CommandSubmission
	if statusComplete {
		submission, err = h.service.ChangeStatus(request.Context(), accounts.ChangeAccountStatusCommand{
			TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrelationID: middleware.CorrelationID(request.Context()),
			IdempotencyKey: request.Header.Get("Idempotency-Key"), AccountID: accountID, ExpectedVersion: expectedVersion,
			TargetStatus: accountdomain.Status(*input.TargetStatus),
			Reason:       *input.Reason,
		})
	} else {
		submission, err = h.service.UpdateMetadata(request.Context(), accounts.UpdateAccountMetadataCommand{
			TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, CorrelationID: middleware.CorrelationID(request.Context()),
			IdempotencyKey: request.Header.Get("Idempotency-Key"), AccountID: accountID, ExpectedVersion: expectedVersion,
			DisplayName: *input.DisplayName, Reference: *input.ExternalReference, Category: *input.Category,
		})
	}
	if err != nil {
		httptransport.WriteError(writer, request, publicAccountCommandError(err))
		return
	}
	writeAccountCommandResult(writer, submission, http.StatusOK)
}

func (h *AccountCommandHandler) authorize(writer http.ResponseWriter, request *http.Request, route string) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("account command handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return identity.Principal{}, false
	}
	if identity.RequireScope(principal, "accounts:write") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "accounts:write")
		return identity.Principal{}, false
	}
	if !enforceTenantCapacity(writer, request, h.rateLimiter, principal, route, h.capacityLimit) {
		return identity.Principal{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, h.rateLimit, true) {
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *AccountCommandHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func decodeAccountCommandRequest(writer http.ResponseWriter, request *http.Request, destination any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errUnsupportedAccountCommandMediaType
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxAccountCommandBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request contains more than one JSON value")
	}
	return nil
}

func writeAccountCommandDecodeError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, errUnsupportedAccountCommandMediaType) {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "Content-Type must be application/json."})
		return
	}
	httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
}

func parseExpectedVersion(value string) (int64, error) {
	if value == "" || value[0] == '0' {
		return 0, errors.New("expected version must be a canonical positive decimal string")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, errors.New("expected version must be a canonical positive decimal string")
		}
	}
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version < 1 {
		return 0, errors.New("expected version must be a canonical positive decimal string")
	}
	return version, nil
}

type accountCommandResponse struct {
	AccountID         string `json:"account_id"`
	TenantID          string `json:"tenant_id"`
	Currency          string `json:"currency"`
	Status            string `json:"status"`
	DisplayName       string `json:"display_name"`
	ExternalReference string `json:"external_reference"`
	Category          string `json:"category"`
	AccountVersion    string `json:"account_version"`
	AvailableMinor    string `json:"available_minor"`
	LedgerMinor       string `json:"ledger_minor"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func writeAccountCommandResult(writer http.ResponseWriter, submission accounts.CommandSubmission, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	if submission.Replayed {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writer.WriteHeader(status)
	result := submission.Result
	_ = json.NewEncoder(writer).Encode(accountCommandResponse{
		AccountID: result.AccountID, TenantID: result.TenantID, Currency: result.Currency, Status: result.Status,
		DisplayName: result.DisplayName, ExternalReference: result.Reference, Category: result.Category, AccountVersion: result.Version,
		AvailableMinor: result.AvailableMinor, LedgerMinor: result.LedgerMinor, CreatedAt: result.CreatedAt, UpdatedAt: result.UpdatedAt,
	})
}

func publicAccountCommandError(err error) error {
	switch {
	case errors.Is(err, accounts.ErrInvalidCommand), errors.Is(err, idempotency.ErrInvalidKey):
		return httptransport.ErrBadRequest
	case errors.Is(err, accounts.ErrAccountNotFound):
		return httptransport.ErrNotFound
	case errors.Is(err, accounts.ErrAccountConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "external_reference_conflict", Message: "The external reference already belongs to another account."}
	case errors.Is(err, accounts.ErrIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "This idempotency key belongs to a different request."}
	case errors.Is(err, accounts.ErrCommandInProgress):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "request_in_progress", Message: "A matching request is still being completed. Retry with the same idempotency key."}
	case errors.Is(err, accounts.ErrVersionConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "account_version_conflict", Message: "The account changed before this command was applied."}
	case errors.Is(err, accounts.ErrInvalidTransition), errors.Is(err, accounts.ErrTerminalStatus):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "invalid_account_transition", Message: "The requested account status transition is not allowed."}
	case errors.Is(err, accounts.ErrNonZeroClose):
		return &httptransport.PublicError{Status: http.StatusUnprocessableEntity, Code: "account_not_zero", Message: "The account must have exact zero available and ledger balances before closing."}
	case errors.Is(err, accounts.ErrFinancialUnavailable):
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "temporary_unavailable", Message: "The account financial state cannot be proven safe for closing. Retry the identical request with the same idempotency key."}
	case errors.Is(err, accounts.ErrOperationalObligations):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "account_obligations_unresolved", Message: "Resolve or cancel pending financial commands before closing this account."}
	case errors.Is(err, accounts.ErrCommandUnavailable), db.IsRetryableTransactionError(err):
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "temporary_unavailable", Message: "The account command outcome is unavailable. Retry the identical request with the same idempotency key."}
	default:
		return err
	}
}
