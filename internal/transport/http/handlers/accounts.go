package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	accountID := strings.TrimSpace(request.PathValue("accountID"))
	if accountID != "" {
		item, getErr := h.service.GetOwned(request.Context(), principal.TenantID, principal.SubjectID, accountID)
		if errors.Is(getErr, accounts.ErrAccountNotFound) {
			httptransport.WriteError(writer, request, httptransport.ErrNotFound)
			return
		}
		if getErr != nil {
			httptransport.WriteError(writer, request, publicAccountError(getErr))
			return
		}
		writeAccountResponse(writer, item)
		return
	}
	limit := 25
	if raw := request.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "The account query is invalid."})
			return
		}
	}
	page, err := h.service.ListOwned(request.Context(), principal.TenantID, principal.SubjectID, accounts.Query{Cursor: request.URL.Query().Get("cursor"), Limit: limit, Search: request.URL.Query().Get("q"), Status: request.URL.Query().Get("status"), Category: request.URL.Query().Get("category")})
	if err != nil {
		if errors.Is(err, accounts.ErrInvalidQuery) {
			httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "The account query is invalid."})
			return
		}
		httptransport.WriteError(writer, request, publicAccountError(err))
		return
	}
	response := make([]accountResponse, 0, len(page.Accounts))
	for _, item := range page.Accounts {
		response = append(response, mapAccountResponse(item))
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]any{"accounts": response, "next_cursor": page.NextCursor})
}

func publicAccountError(err error) error {
	if errors.Is(err, accounts.ErrAccountDirectoryUnavailable) {
		return &httptransport.PublicError{
			Status:  http.StatusServiceUnavailable,
			Code:    "account_directory_unavailable",
			Message: "The account directory is temporarily unavailable. No empty result is being inferred.",
		}
	}
	return err
}

type accountResponse struct {
	AccountID         string `json:"account_id"`
	AccountVersion    string `json:"account_version"`
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

func mapAccountResponse(item accounts.Summary) accountResponse {
	return accountResponse{AccountID: item.AccountID, AccountVersion: strconv.FormatInt(item.AccountVersion, 10), Currency: item.Currency, Status: item.Status, AvailableMinor: strconv.FormatInt(item.Balance.AvailableMinor, 10), LedgerMinor: strconv.FormatInt(item.Balance.LedgerMinor, 10), Version: strconv.FormatInt(item.Balance.Version, 10), AsOf: item.Balance.AsOf.Format(time.RFC3339Nano), DisplayName: item.DisplayName, Category: item.Category, ExternalReference: item.ExternalReference}
}

func writeAccountResponse(writer http.ResponseWriter, item accounts.Summary) {
	type auditResponse struct {
		EventID        string `json:"event_id"`
		EventType      string `json:"event_type"`
		ActorSubjectID string `json:"actor_subject_id"`
		Outcome        string `json:"outcome"`
		CorrelationID  string `json:"correlation_id"`
		Reason         string `json:"reason,omitempty"`
		OccurredAt     string `json:"occurred_at"`
	}
	audit := make([]auditResponse, 0, len(item.AuditContext))
	for _, event := range item.AuditContext {
		audit = append(audit, auditResponse{EventID: event.EventID, EventType: event.EventType, ActorSubjectID: event.ActorSubjectID, Outcome: event.Outcome, CorrelationID: event.CorrelationID, Reason: event.Reason, OccurredAt: event.OccurredAt.Format(time.RFC3339Nano)})
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(struct {
		accountResponse
		AuditContext []auditResponse `json:"audit_context"`
	}{accountResponse: mapAccountResponse(item), AuditContext: audit})
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
