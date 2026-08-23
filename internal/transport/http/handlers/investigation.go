package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

type InvestigationHandler struct {
	repository    investigation.Repository
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
}

func NewInvestigationHandler(repository investigation.Repository, provider identity.Provider) *InvestigationHandler {
	return &InvestigationHandler{repository: repository, identity: provider}
}
func (h *InvestigationHandler) WithBFFAssertionSecret(secret string) *InvestigationHandler {
	if authenticator, err := identity.NewRequestAuthenticator(h.identity, secret); err == nil {
		h.authenticator = authenticator
	}
	return h
}
func (h *InvestigationHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func (h *InvestigationHandler) Transfers(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "transfers:read")
	if !ok {
		return
	}
	limit, ok := boundedLimit(writer, request, 25)
	if !ok {
		return
	}
	filter := investigation.TransferFilter{Cursor: request.URL.Query().Get("cursor"), AccountID: request.URL.Query().Get("accountId"), Status: request.URL.Query().Get("status"), Query: request.URL.Query().Get("q"), Limit: limit}
	var err error
	if raw := request.URL.Query().Get("from"); raw != "" {
		filter.From, err = time.Parse(time.RFC3339, raw)
	}
	if err == nil {
		if raw := request.URL.Query().Get("to"); raw != "" {
			filter.To, err = time.Parse(time.RFC3339, raw)
		}
	}
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	items, next, err := h.repository.ListTransfers(request.Context(), principal.TenantID, filter)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	writeInvestigationJSON(writer, map[string]any{"transfers": items, "next_cursor": next})
}

func (h *InvestigationHandler) Transfer(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "transfers:read")
	if !ok {
		return
	}
	id := strings.TrimSpace(request.PathValue("transferID"))
	if id == "" {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	item, err := h.repository.GetTransfer(request.Context(), principal.TenantID, id)
	if errors.Is(err, db.ErrInvestigationNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		httptransport.WriteError(writer, request, err)
		return
	}
	writeInvestigationJSON(writer, item)
}

func (h *InvestigationHandler) ReconciliationRuns(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "reconciliation:read")
	if !ok {
		return
	}
	limit, ok := boundedLimit(writer, request, 25)
	if !ok {
		return
	}
	items, next, err := h.repository.ListReconciliationRuns(request.Context(), principal.TenantID, request.URL.Query().Get("cursor"), limit)
	if err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Authoritative reconciliation evidence is unavailable."})
		return
	}
	writeInvestigationJSON(writer, map[string]any{"runs": items, "next_cursor": next})
}
func (h *InvestigationHandler) ReconciliationRun(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "reconciliation:read")
	if !ok {
		return
	}
	item, err := h.repository.GetReconciliationRun(request.Context(), principal.TenantID, strings.TrimSpace(request.PathValue("runID")))
	if errors.Is(err, db.ErrInvestigationNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Authoritative reconciliation evidence is unavailable."})
		return
	}
	writeInvestigationJSON(writer, item)
}

func (h *InvestigationHandler) authorize(writer http.ResponseWriter, request *http.Request, scope string) (identity.Principal, bool) {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("investigation handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return identity.Principal{}, false
	}
	if identity.RequireScope(principal, scope) != nil {
		httptransport.WriteError(writer, request, httptransport.ErrForbidden)
		return identity.Principal{}, false
	}
	return principal, true
}
func boundedLimit(writer http.ResponseWriter, request *http.Request, fallback int) (int, bool) {
	limit := fallback
	if raw := request.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}
func writeInvestigationJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(value)
}
