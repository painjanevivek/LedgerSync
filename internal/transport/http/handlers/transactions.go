package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

type TransactionsHandler struct {
	history       *transactions.History
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
}

func (h *TransactionsHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *TransactionsHandler {
	h.authenticator = authenticator
	return h
}

func (h *TransactionsHandler) WithRateLimiter(limiter RateLimiter, requestsPerMinute int) *TransactionsHandler {
	h.rateLimiter, h.rateLimit = limiter, requestsPerMinute
	return h
}
func (h *TransactionsHandler) WithAuditRecorder(audit AuditRecorder) *TransactionsHandler {
	h.audit = audit
	return h
}

func NewTransactionsHandler(history *transactions.History, provider identity.Provider) *TransactionsHandler {
	return &TransactionsHandler{history: history, identity: provider}
}

func (h *TransactionsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only GET is allowed."})
		return
	}
	if h == nil || h.history == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("transactions handler is not configured"))
		return
	}
	accountID := strings.TrimSpace(request.PathValue("accountID"))
	if accountID == "" {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	principal, err := h.authenticate(request)
	if err != nil {
		writeAuthenticationError(writer, request, err)
		return
	}
	canonicalAccountID, ok := requireCanonicalIdentifier(writer, request, identifier.KindAccount, accountID)
	if !ok {
		return
	}
	accountID = canonicalAccountID
	if identity.RequireScope(principal, "transactions:read") != nil {
		writeScopeDenial(writer, request, h.audit, principal, "transactions:read")
		return
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "transactions:list", h.rateLimit, false) {
		return
	}
	limit := 50
	if raw := request.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
			return
		}
	}
	entries, next, err := h.history.List(request.Context(), principal.TenantID, principal.SubjectID, accountID, request.URL.Query().Get("cursor"), limit)
	if errors.Is(err, transactions.ErrHistoryNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]any{"transactions": entries, "next_cursor": next})
}
func (h *TransactionsHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}
