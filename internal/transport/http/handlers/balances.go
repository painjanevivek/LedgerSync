package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

const consistencyRequirementHeader = "X-LedgerSync-Consistency-Requirement"

type BalanceHandler struct {
	reader        *accounts.Reader
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
}

func (h *BalanceHandler) WithBFFAssertionSecret(secret string) *BalanceHandler {
	if authenticator, err := identity.NewRequestAuthenticator(h.identity, secret); err == nil {
		h.authenticator = authenticator
	}
	return h
}

func (h *BalanceHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *BalanceHandler {
	h.authenticator = authenticator
	return h
}

func NewBalanceHandler(reader *accounts.Reader, provider identity.Provider) *BalanceHandler {
	return &BalanceHandler{reader: reader, identity: provider}
}

func (h *BalanceHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only GET is allowed."})
		return
	}
	if h == nil || h.reader == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("balance handler is not configured"))
		return
	}
	accountID, ok := balanceAccountID(request)
	if !ok {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return
	}
	if identity.RequireScope(principal, "accounts:read") != nil {
		httptransport.WriteError(writer, request, httptransport.ErrForbidden)
		return
	}
	result, err := h.reader.Read(request.Context(), principal.TenantID, principal.SubjectID, accountID, request.Header.Get(consistencyRequirementHeader))
	if err != nil {
		httptransport.WriteError(writer, request, publicBalanceError(err))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(balanceResponse{AccountID: result.Balance.AccountID, Currency: result.Balance.Currency, AvailableMinor: strconv.FormatInt(result.Balance.AvailableMinor, 10), LedgerMinor: strconv.FormatInt(result.Balance.LedgerMinor, 10), Version: strconv.FormatInt(result.Balance.Version, 10), AsOf: result.Balance.AsOf.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
}
func (h *BalanceHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

type balanceResponse struct {
	AccountID      string `json:"account_id"`
	Currency       string `json:"currency"`
	AvailableMinor string `json:"available_minor"`
	LedgerMinor    string `json:"ledger_minor"`
	Version        string `json:"version"`
	AsOf           string `json:"as_of"`
}

func balanceAccountID(request *http.Request) (string, bool) {
	if accountID := strings.TrimSpace(request.PathValue("accountID")); accountID != "" {
		return accountID, true
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "accounts" && parts[3] == "balance" && strings.TrimSpace(parts[2]) != "" {
		return parts[2], true
	}
	return "", false
}

func publicBalanceError(err error) error {
	switch {
	case errors.Is(err, db.ErrBalanceNotAuthorized):
		return httptransport.ErrNotFound
	case errors.Is(err, accounts.ErrCurrentBalanceUnavailable):
		return &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "current_balance_unavailable", Message: "Current balance is temporarily unavailable."}
	case errors.Is(err, consistency.ErrInvalidRequirement), errors.Is(err, consistency.ErrExpiredRequirement):
		return httptransport.ErrBadRequest
	default:
		return err
	}
}
