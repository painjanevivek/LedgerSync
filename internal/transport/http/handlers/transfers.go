// Package handlers contains HTTP adapters. It translates authenticated JSON
// requests to application commands and deliberately contains no SQL or ledger
// calculations.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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
	service  *transfers.Service
	identity identity.Provider
	issuer   *consistency.Issuer
}

func NewTransferHandler(service *transfers.Service, provider identity.Provider, issuers ...*consistency.Issuer) *TransferHandler {
	var issuer *consistency.Issuer
	if len(issuers) > 0 {
		issuer = issuers[0]
	}
	return &TransferHandler{service: service, identity: provider, issuer: issuer}
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
	principal, err := h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
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
		requirements := make(map[string]string, len(submission.Result.MinimumBalanceVersions))
		for accountID, version := range submission.Result.MinimumBalanceVersions {
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
	case errors.Is(err, db.ErrAccountNotFound), errors.Is(err, db.ErrNotAuthorized):
		// Return a safe denial/no-disclosure response for inaccessible accounts.
		return httptransport.ErrForbidden
	case errors.Is(err, db.ErrAccountInactive):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "account_inactive", Message: "An account is not active for this transfer."}
	case errors.Is(err, db.ErrInsufficientFunds):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "insufficient_funds", Message: "The source account has insufficient posted balance."}
	case errors.Is(err, transfers.ErrIdempotencyConflict):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "idempotency_conflict", Message: "This idempotency key belongs to a different request."}
	case errors.Is(err, transfers.ErrRequestInProgress):
		return &httptransport.PublicError{Status: http.StatusConflict, Code: "request_in_progress", Message: "A matching request is still being completed. Retry with the same idempotency key."}
	default:
		return err
	}
}
