package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

type AccountsHandler struct {
	service       *accounts.Service
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
}

func (h *AccountsHandler) WithBFFAssertionSecret(secret string) *AccountsHandler {
	if authenticator, err := identity.NewRequestAuthenticator(h.identity, secret); err == nil {
		h.authenticator = authenticator
	}
	return h
}

func (h *AccountsHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *AccountsHandler {
	h.authenticator = authenticator
	return h
}

func (h *AccountsHandler) WithRateLimiter(limiter RateLimiter, requestsPerMinute int) *AccountsHandler {
	h.rateLimiter, h.rateLimit = limiter, requestsPerMinute
	return h
}
func (h *AccountsHandler) WithAuditRecorder(audit AuditRecorder) *AccountsHandler {
	h.audit = audit
	return h
}

func NewAccountsHandler(service *accounts.Service, provider identity.Provider) *AccountsHandler {
	return &AccountsHandler{service: service, identity: provider}
}

func (h *AccountsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only GET is allowed."})
		return
	}
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("accounts handler is not configured"))
		return
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return
	}
	if identity.RequireScope(principal, "accounts:read") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "accounts:read")
		return
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "accounts:list", h.rateLimit, false) {
		return
	}
	items, err := h.service.ListOwned(request.Context(), principal.TenantID, principal.SubjectID)
	if err != nil {
		httptransport.WriteError(writer, request, err)
		return
	}
	type accountResponse struct {
		AccountID         string `json:"account_id"`
		Currency          string `json:"currency"`
		Status            string `json:"status"`
		AvailableMinor    string `json:"available_minor"`
		LedgerMinor       string `json:"ledger_minor"`
		Version           string `json:"version"`
		AsOf              string `json:"as_of"`
		DisplayName       string `json:"display_name,omitempty"`
		Category          string `json:"category,omitempty"`
		ExternalReference string `json:"external_reference,omitempty"`
	}
	response := make([]accountResponse, 0, len(items))
	for _, item := range items {
		response = append(response, accountResponse{AccountID: item.AccountID, Currency: item.Currency, Status: item.Status, AvailableMinor: strconv.FormatInt(item.Balance.AvailableMinor, 10), LedgerMinor: strconv.FormatInt(item.Balance.LedgerMinor, 10), Version: strconv.FormatInt(item.Balance.Version, 10), AsOf: item.Balance.AsOf.Format("2006-01-02T15:04:05.999999999Z07:00"), DisplayName: item.DisplayName, Category: item.Category, ExternalReference: item.ExternalReference})
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]any{"accounts": response})
}
func (h *AccountsHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}
