package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

var canonicalInvestigationUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var boundedTransferIDQuery = regexp.MustCompile(`^[0-9A-Fa-f-]{1,128}$`)
var boundedExactInvestigationReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{7,127}$`)
var boundedInvestigationSearchLimit = regexp.MustCompile(`^(?:[1-9]|1[0-9]|20)$`)

type InvestigationHandler struct {
	repository          investigation.Repository
	identity            identity.Provider
	authenticator       *identity.RequestAuthenticator
	rateLimiter         RateLimiter
	rateLimit           int
	savedViewWriteLimit int
	audit               AuditRecorder
}

func NewInvestigationHandler(repository investigation.Repository, provider identity.Provider) *InvestigationHandler {
	return &InvestigationHandler{repository: repository, identity: provider}
}
func (h *InvestigationHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *InvestigationHandler {
	h.authenticator = authenticator
	return h
}
func (h *InvestigationHandler) WithRateLimiter(limiter RateLimiter, requestsPerMinute int) *InvestigationHandler {
	h.rateLimiter, h.rateLimit = limiter, requestsPerMinute
	return h
}
func (h *InvestigationHandler) WithSavedViewWriteLimit(requestsPerMinute int) *InvestigationHandler {
	h.savedViewWriteLimit = requestsPerMinute
	return h
}
func (h *InvestigationHandler) WithAuditRecorder(audit AuditRecorder) *InvestigationHandler {
	h.audit = audit
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

func (h *InvestigationHandler) Search(writer http.ResponseWriter, request *http.Request) {
	principal, access, ok := h.authorizeSearch(writer, request)
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "q", "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	queryValues, limitValues := request.URL.Query()["q"], request.URL.Query()["limit"]
	if len(queryValues) != 1 || len(limitValues) > 1 {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	query := queryValues[0]
	if query == "" || query != strings.TrimSpace(query) || len(query) > 128 {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	kind := "approved_reference"
	if canonicalInvestigationUUID.MatchString(strings.ToLower(query)) {
		query = strings.ToLower(query)
		kind = "immutable_id"
	} else if !boundedExactInvestigationReference.MatchString(query) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	limit := 10
	if raw := request.URL.Query().Get("limit"); raw != "" {
		if !boundedInvestigationSearchLimit.MatchString(raw) {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
			return
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 20 {
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
			return
		}
		limit = parsed
	}
	page, err := h.repository.Search(request.Context(), principal.TenantID, principal.SubjectID, investigation.SearchFilter{Query: query, QueryKind: kind, Limit: limit, Access: access})
	if err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Authorized search evidence is unavailable."})
		return
	}
	writeInvestigationJSON(writer, page)
}

func (h *InvestigationHandler) Related(writer http.ResponseWriter, request *http.Request) {
	principal, access, ok := h.authorizeRelationships(writer, request)
	if !ok {
		return
	}
	if !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	sourceType := strings.TrimSpace(request.PathValue("recordType"))
	sourceID := strings.ToLower(strings.TrimSpace(request.PathValue("recordId")))
	if !canonicalInvestigationUUID.MatchString(sourceID) || !relationshipSourceType(sourceType) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	repository, configured := h.repository.(investigation.RelationshipRepository)
	if !configured {
		httptransport.WriteError(writer, request, errors.New("investigation relationship repository is not configured"))
		return
	}
	page, err := repository.Related(request.Context(), principal.TenantID, principal.SubjectID, investigation.RelationshipFilter{SourceType: sourceType, SourceID: sourceID, Limit: 20, Access: access})
	if errors.Is(err, db.ErrInvestigationNotFound) {
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
		return
	}
	if err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "evidence_unavailable", Message: "Related investigation evidence is unavailable."})
		return
	}
	writeInvestigationJSON(writer, page)
}

func relationshipSourceType(value string) bool {
	switch value {
	case "account", "transfer", "funding", "event", "reconciliation_run", "reconciliation_mismatch", "correction":
		return true
	default:
		return false
	}
}

func (h *InvestigationHandler) authorizeRelationships(writer http.ResponseWriter, request *http.Request) (identity.Principal, investigation.RelationshipAccess, bool) {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("investigation handler is not configured"))
		return identity.Principal{}, investigation.RelationshipAccess{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return identity.Principal{}, investigation.RelationshipAccess{}, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:investigate")
		return identity.Principal{}, investigation.RelationshipAccess{}, false
	}
	if !principal.HasScope("investigation:read") {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.RelationshipAccess{}, false
	}
	access := investigation.RelationshipAccess{
		Accounts: principal.HasScope("accounts:read"), Transfers: principal.HasScope("transfers:read"),
		Funding: principal.HasScope("funding:read"), Events: principal.HasScope("events:read"),
		Reconciliation: principal.HasScope("reconciliation:read"), Corrections: principal.HasScope("corrections:read"),
	}
	if !access.Any() {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.RelationshipAccess{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "investigation:relationships", h.rateLimit, false) {
		return identity.Principal{}, investigation.RelationshipAccess{}, false
	}
	return principal, access, true
}

func (h *InvestigationHandler) authorizeSearch(writer http.ResponseWriter, request *http.Request) (identity.Principal, investigation.SearchAccess, bool) {
	if h == nil || h.repository == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("investigation handler is not configured"))
		return identity.Principal{}, investigation.SearchAccess{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return identity.Principal{}, investigation.SearchAccess{}, false
	}
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:investigate")
		return identity.Principal{}, investigation.SearchAccess{}, false
	}
	if !principal.HasScope("investigation:read") {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.SearchAccess{}, false
	}
	access := investigation.SearchAccess{
		Accounts: principal.HasScope("accounts:read"), Transfers: principal.HasScope("transfers:read"),
		Funding: principal.HasScope("funding:read"), Events: principal.HasScope("events:read"),
		Reconciliation: principal.HasScope("reconciliation:read"), Corrections: principal.HasScope("corrections:read"),
	}
	if !access.Any() {
		writeScopeDenial(writer, request, h.audit, principal, "investigation:read")
		return identity.Principal{}, investigation.SearchAccess{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "investigation:search", h.rateLimit, false) {
		return identity.Principal{}, investigation.SearchAccess{}, false
	}
	return principal, access, true
}

func (h *InvestigationHandler) Transfers(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, "transfers:read")
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "cursor", "limit", "accountId", "status", "q", "from", "to") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	limit, ok := boundedLimit(writer, request, 25)
	if !ok {
		return
	}
	filter := investigation.TransferFilter{
		Cursor:    strings.TrimSpace(request.URL.Query().Get("cursor")),
		AccountID: strings.ToLower(strings.TrimSpace(request.URL.Query().Get("accountId"))),
		Status:    strings.TrimSpace(request.URL.Query().Get("status")),
		Query:     strings.TrimSpace(request.URL.Query().Get("q")),
		Limit:     limit,
	}
	if filter.AccountID != "" && !canonicalInvestigationUUID.MatchString(filter.AccountID) ||
		filter.Status != "" && filter.Status != "pending" && filter.Status != "posted" && filter.Status != "rejected" ||
		filter.Query != "" && !boundedTransferIDQuery.MatchString(filter.Query) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	var err error
	if raw := request.URL.Query().Get("from"); raw != "" {
		filter.From, err = time.Parse(time.RFC3339, raw)
	}
	if err == nil {
		if raw := request.URL.Query().Get("to"); raw != "" {
			filter.To, err = time.Parse(time.RFC3339, raw)
		}
	}
	if err != nil || !filter.From.IsZero() && !filter.To.IsZero() && filter.From.After(filter.To) {
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
		writeScopeDenial(writer, request, h.audit, principal, scope)
		return identity.Principal{}, false
	}
	// Investigation evidence is tenant-wide by product contract. A narrow read
	// scope alone must never silently become administrative access.
	if !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:investigate")
		return identity.Principal{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, scope, h.rateLimit, false) {
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
