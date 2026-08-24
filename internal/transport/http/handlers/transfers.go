// Package handlers contains HTTP adapters. It translates authenticated JSON
// requests to application commands and deliberately contains no SQL or ledger
// calculations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const maxTransferBodyBytes = 64 * 1024

type TransferHandler struct {
	service       *transfers.Service
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	issuer        *consistency.Issuer
	balanceReader consistencyBalanceReader
	rateLimiter   RateLimiter
	rateLimit     int
	capacityLimit int
	audit         AuditRecorder
}

type consistencyBalanceReader interface {
	ReadCurrent(context.Context, string, string, string) (accounts.Balance, error)
}

func NewTransferHandler(service *transfers.Service, provider identity.Provider, issuers ...*consistency.Issuer) *TransferHandler {
	var issuer *consistency.Issuer
	if len(issuers) > 0 {
		issuer = issuers[0]
	}
	return &TransferHandler{service: service, identity: provider, issuer: issuer}
}

func (h *TransferHandler) WithBFFAssertionSecret(secret string) *TransferHandler {
	if authenticator, err := identity.NewRequestAuthenticator(h.identity, secret); err == nil {
		h.authenticator = authenticator
	}
	return h
}

func (h *TransferHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *TransferHandler {
	h.authenticator = authenticator
	return h
}

// WithConsistencyBalanceReader lets the handler add a private read-your-writes
// requirement for an owned destination account. The destination balance and
// version remain absent from the public transfer result; credit-only actors do
// not receive a requirement for accounts they cannot read.
func (h *TransferHandler) WithConsistencyBalanceReader(reader consistencyBalanceReader) *TransferHandler {
	h.balanceReader = reader
	return h
}

func (h *TransferHandler) WithRateLimiter(limiter RateLimiter, requestsPerMinute int) *TransferHandler {
	h.rateLimiter, h.rateLimit = limiter, requestsPerMinute
	return h
}

// WithCapacityLimit applies the measured pilot envelope across all actors in a
// tenant. It is separate from the per-principal abuse limit so adding API
// clients cannot bypass the financial-system capacity boundary.
func (h *TransferHandler) WithCapacityLimit(limiter RateLimiter, requestsPerSecond int) *TransferHandler {
	h.rateLimiter, h.capacityLimit = limiter, requestsPerSecond
	return h
}
func (h *TransferHandler) WithAuditRecorder(audit AuditRecorder) *TransferHandler {
	h.audit = audit
	return h
}

type createTransferRequest struct {
	SourceAccountID      string `json:"source_account_id"`
	DestinationAccountID string `json:"destination_account_id"`
	Amount               string `json:"amount"`
	Currency             string `json:"currency"`
}

func (h *TransferHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only POST is allowed."})
		return
	}
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("transfer handler is not configured"))
		return
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return
	}
	if identity.RequireScope(principal, "transfers:write") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "transfers:write")
		return
	}
	if !enforceTenantCapacity(writer, request, h.rateLimiter, principal, "transfers:create", h.capacityLimit) {
		return
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "transfers:create", h.rateLimit, true) {
		return
	}
	input, err := decodeTransferRequest(writer, request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	amount, err := money.Parse(input.Currency, input.Amount)
	if err != nil || !amount.IsPositive() {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	submission, err := h.service.Submit(request.Context(), transfers.Command{
		TenantID:        principal.TenantID,
		ActorSubjectID:  principal.SubjectID,
		DebitAccountID:  input.SourceAccountID,
		CreditAccountID: input.DestinationAccountID,
		Amount:          amount,
		IdempotencyKey:  request.Header.Get("Idempotency-Key"),
		CorrelationID:   middleware.CorrelationID(request.Context()),
	})
	if err != nil {
		httptransport.WriteError(writer, request, publicTransferError(err))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	if submission.Replayed {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	if h.issuer != nil && submission.Result.Status == "posted" {
		versions := make(map[string]int64, len(submission.Result.MinimumBalanceVersions)+1)
		for accountID, version := range submission.Result.MinimumBalanceVersions {
			versions[accountID] = version
		}
		if h.balanceReader != nil {
			if destination, readErr := h.balanceReader.ReadCurrent(request.Context(), principal.TenantID, principal.SubjectID, input.DestinationAccountID); readErr == nil {
				versions[destination.AccountID] = destination.Version
			}
		}
		requirements := make(map[string]string, len(versions))
		for accountID, version := range versions {
			requirement, err := h.issuer.Issue(principal.TenantID, accountID, version)
			if err != nil {
				httptransport.WriteError(writer, request, err)
				return
			}
			requirements[accountID] = requirement
		}
		encoded, err := json.Marshal(requirements)
		if err != nil {
			httptransport.WriteError(writer, request, err)
			return
		}
		writer.Header().Set("X-LedgerSync-Consistency-Requirements", string(encoded))
	}
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(submission.Result)
}

func (h *TransferHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func decodeTransferRequest(writer http.ResponseWriter, request *http.Request) (createTransferRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxTransferBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input createTransferRequest
	if err := decoder.Decode(&input); err != nil {
		return createTransferRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return createTransferRequest{}, errors.New("request contains more than one JSON value")
	}
	if strings.TrimSpace(input.SourceAccountID) == "" || strings.TrimSpace(input.DestinationAccountID) == "" || strings.TrimSpace(input.Amount) == "" || strings.TrimSpace(input.Currency) == "" {
		return createTransferRequest{}, errors.New("required field is missing")
	}
	return input, nil
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func publicTransferError(err error) error {
	switch {
	case errors.Is(err, transfers.ErrInvalidCommand), errors.Is(err, transfers.ErrInvalidIdempotencyKey), errors.Is(err, money.ErrInvalidAmount), errors.Is(err, money.ErrUnsupportedCurrency), errors.Is(err, money.ErrCurrencyMismatch):
		return httptransport.ErrBadRequest
	case errors.Is(err, db.ErrAccountNotFound), errors.Is(err, db.ErrNotAuthorized), errors.Is(err, db.ErrDestinationNotAuthorized):
		// Return a safe denial/no-disclosure response for inaccessible accounts.
		return httptransport.ErrForbidden
	case errors.Is(err, db.ErrAccountInactive):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "account_inactive", Message: "An account is not active for this transfer."}
	case errors.Is(err, db.ErrInsufficientFunds):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "insufficient_funds", Message: "The source account has insufficient posted balance."}
	case errors.Is(err, db.ErrTransferBelowMinimum), errors.Is(err, db.ErrTransferAboveMaximum), errors.Is(err, db.ErrActorVelocityExceeded), errors.Is(err, db.ErrSourceVelocityExceeded), errors.Is(err, db.ErrTenantVelocityExceeded):
		return &httptransport.PublicError{Status: http.StatusUnprocessableEntity, Code: "transfer_policy_denied", Message: "The transfer is outside the tenant's approved amount or rolling limits."}
	case errors.Is(err, db.ErrTenantPolicyMissing), errors.Is(err, db.ErrUnsupportedPilotCurrency):
		return &httptransport.PublicError{Status: http.StatusUnprocessableEntity, Code: "unsupported_pilot_policy", Message: "The transfer is outside the configured pilot currency or tenant policy."}
	case errors.Is(err, transfers.ErrIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "This idempotency key belongs to a different request."}
	case errors.Is(err, transfers.ErrRequestInProgress):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "request_in_progress", Message: "A matching request is still being completed. Retry with the same idempotency key."}
	case db.IsRetryableTransactionError(err):
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "transaction_conflict_retryable", Message: "The transfer could not acquire a safe commit window. Retry the identical request with the same idempotency key."}
	default:
		return err
	}
}
