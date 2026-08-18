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
}

func (h *AccountsHandler) WithBFFAssertionSecret(secret string) *AccountsHandler {
	if authenticator, err := identity.NewRequestAuthenticator(h.identity, secret); err == nil {
		h.authenticator = authenticator
	}
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
		httptransport.WriteError(writer, request, httptransport.ErrForbidden)
		return
	}
	items, err := h.service.ListOwned(request.Context(), principal.TenantID, principal.SubjectID)
	if err != nil {
		httptransport.WriteError(writer, request, err)
		return
	}
	type accountResponse struct {
		AccountID      string `json:"account_id"`
		Currency       string `json:"currency"`
		Status         string `json:"status"`
		AvailableMinor string `json:"available_minor"`
		LedgerMinor    string `json:"ledger_minor"`
		Version        string `json:"version"`
		AsOf           string `json:"as_of"`
	}
	response := make([]accountResponse, 0, len(items))
	for _, item := range items {
		response = append(response, accountResponse{item.AccountID, item.Currency, item.Status, strconv.FormatInt(item.Balance.AvailableMinor, 10), strconv.FormatInt(item.Balance.LedgerMinor, 10), strconv.FormatInt(item.Balance.Version, 10), item.Balance.AsOf.Format("2006-01-02T15:04:05.999999999Z07:00")})
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
