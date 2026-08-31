package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

type OperationsHandler struct {
	diagnostics   *operations.DiagnosticService
	events        *operations.EventService
	webhooks      *operations.WebhookEndpointService
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
}

func NewOperationsHandler(diagnostics *operations.DiagnosticService, events *operations.EventService, webhooks *operations.WebhookEndpointService, provider identity.Provider) *OperationsHandler {
	return &OperationsHandler{diagnostics: diagnostics, events: events, webhooks: webhooks, identity: provider}
}
func (h *OperationsHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *OperationsHandler {
	h.authenticator = authenticator
	return h
}
func (h *OperationsHandler) WithRateLimiter(limiter RateLimiter, limit int) *OperationsHandler {
	h.rateLimiter, h.rateLimit = limiter, limit
	return h
}
func (h *OperationsHandler) WithAuditRecorder(audit AuditRecorder) *OperationsHandler {
	h.audit = audit
	return h
}

func (h *OperationsHandler) Diagnostics(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return
	}
	principal, ok := h.authorize(writer, request, "local:read", "local:diagnostics")
	if !ok {
		return
	}
	writeInvestigationJSON(writer, h.diagnostics.Snapshot(request.Context(), principal.TenantID))
}

func (h *OperationsHandler) Events(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return
	}
	principal, ok := h.authorize(writer, request, "events:read", "events:list")
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "cursor", "eventType", "state", "endpointId", "relatedId", "correlationId", "from", "to", "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	limit, ok := boundedLimit(writer, request, 25)
	if !ok {
		return
	}
	filter := operations.EventFilter{Cursor: request.URL.Query().Get("cursor"), EventType: request.URL.Query().Get("eventType"), State: request.URL.Query().Get("state"), EndpointID: request.URL.Query().Get("endpointId"), RelatedID: request.URL.Query().Get("relatedId"), CorrelationID: request.URL.Query().Get("correlationId"), Limit: limit}
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
	items, next, err := h.events.List(request.Context(), principal.TenantID, principal.SubjectID, filter)
	if err != nil {
		writeOperationsEvidenceError(writer, request, err)
		return
	}
	writeInvestigationJSON(writer, map[string]any{"events": items, "next_cursor": next})
}

func (h *OperationsHandler) WebhookEndpoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return
	}
	principal, ok := h.authorize(writer, request, "webhooks:read", "webhooks:evidence_list")
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "cursor", "status", "eventType", "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	limit, ok := boundedLimit(writer, request, 25)
	if !ok {
		return
	}
	page, err := h.webhooks.List(request.Context(), principal.TenantID, principal.SubjectID, operations.WebhookEndpointFilter{Cursor: request.URL.Query().Get("cursor"), Status: request.URL.Query().Get("status"), EventType: request.URL.Query().Get("eventType"), Limit: limit})
	if err != nil {
		writeOperationsEvidenceError(writer, request, err)
		return
	}
	writeInvestigationJSON(writer, page)
}

func (h *OperationsHandler) WebhookEndpoint(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return
	}
	principal, ok := h.authorize(writer, request, "webhooks:read", "webhooks:evidence_detail")
	if !ok {
		return
	}
	if request.URL.RawQuery != "" {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	item, err := h.webhooks.Get(request.Context(), principal.TenantID, principal.SubjectID, strings.TrimSpace(request.PathValue("endpointId")))
	if errors.Is(err, db.ErrInvestigationNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		writeOperationsEvidenceError(writer, request, err)
		return
	}
	writeInvestigationJSON(writer, item)
}

func (h *OperationsHandler) Event(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return
	}
	principal, ok := h.authorize(writer, request, "events:read", "events:detail")
	if !ok {
		return
	}
	item, err := h.events.Get(request.Context(), principal.TenantID, principal.SubjectID, strings.TrimSpace(request.PathValue("eventID")))
	if errors.Is(err, db.ErrInvestigationNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		writeOperationsEvidenceError(writer, request, err)
		return
	}
	writeInvestigationJSON(writer, item)
}

func (h *OperationsHandler) authorize(writer http.ResponseWriter, request *http.Request, scope, route string) (identity.Principal, bool) {
	if h == nil || h.diagnostics == nil || h.events == nil || h.webhooks == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("operations handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return identity.Principal{}, false
	}
	if identity.RequireScope(principal, scope) != nil {
		writeScopeDenial(writer, request, h.audit, principal, scope)
		return identity.Principal{}, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:operations")
		return identity.Principal{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, h.rateLimit, false) {
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *OperationsHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func onlyQueryParameters(request *http.Request, allowed ...string) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		set[item] = struct{}{}
	}
	for name, values := range request.URL.Query() {
		if _, ok := set[name]; !ok || len(values) != 1 {
			return false
		}
	}
	return true
}

func writeGETOnly(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Allow", http.MethodGet)
	httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Only GET is allowed."})
}

func writeOperationsEvidenceError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, operations.ErrInvalidEventEvidence) || errors.Is(err, operations.ErrInvalidWebhookEvidence) || strings.Contains(err.Error(), "invalid event cursor") || strings.Contains(err.Error(), "invalid webhook endpoint cursor") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Authoritative event evidence is unavailable."})
}
