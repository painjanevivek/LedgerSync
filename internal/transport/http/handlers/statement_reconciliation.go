package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	statement "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/statementreconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const MaxStatementRequestBytes = 1_500_000

type StatementReconciliationHandler struct {
	service                       *statement.Service
	identity                      identity.Provider
	authenticator                 *identity.RequestAuthenticator
	limiter                       RateLimiter
	readRate, writeRate, capacity int
	audit                         AuditRecorder
}

func NewStatementReconciliationHandler(service *statement.Service, provider identity.Provider) *StatementReconciliationHandler {
	return &StatementReconciliationHandler{service: service, identity: provider}
}
func (h *StatementReconciliationHandler) WithRequestAuthenticator(a *identity.RequestAuthenticator) *StatementReconciliationHandler {
	h.authenticator = a
	return h
}
func (h *StatementReconciliationHandler) WithRateLimiter(l RateLimiter, read, write, capacity int) *StatementReconciliationHandler {
	h.limiter = l
	h.readRate = read
	h.writeRate = write
	h.capacity = capacity
	return h
}
func (h *StatementReconciliationHandler) WithAuditRecorder(a AuditRecorder) *StatementReconciliationHandler {
	h.audit = a
	return h
}
func (h *StatementReconciliationHandler) authorize(w http.ResponseWriter, r *http.Request, write bool, operation string) (identity.Principal, bool) {
	if h == nil || h.service == nil || h.identity == nil {
		writeStatementError(w, r, statement.ErrUnavailable)
		return identity.Principal{}, false
	}
	method := http.MethodGet
	if write {
		method = http.MethodPost
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		httptransport.WriteError(w, r, &httptransport.PublicError{Status: 405, Code: "method_not_allowed", Message: "Use the documented method."})
		return identity.Principal{}, false
	}
	var p identity.Principal
	var err error
	assertion := r.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		p, err = h.authenticator.Authenticate(r.Context(), bearerToken(r.Header.Get("Authorization")), assertion)
	} else if assertion != "" {
		err = identity.ErrUnauthenticated
	} else {
		p, err = h.identity.Authenticate(r.Context(), bearerToken(r.Header.Get("Authorization")))
	}
	if err != nil {
		writeAuthenticationError(w, r, err)
		return p, false
	}
	scope, rate := "reconciliation:read", h.readRate
	if write {
		scope, rate = "reconciliation:write", h.writeRate
	}
	if identity.RequireScope(p, scope) != nil {
		writeScopeDenial(w, r, h.audit, p, scope)
		return p, false
	}
	if write && !enforceTenantCapacity(w, r, h.limiter, p, "statement:"+operation, h.capacity) {
		return p, false
	}
	if !enforceRateLimit(w, r, h.limiter, p, "statement:"+operation, rate, true) {
		return p, false
	}
	return p, true
}
func statementPrincipal(p identity.Principal) statement.Scope {
	return statement.Scope{TenantID: p.TenantID, ActorSubjectID: p.SubjectID}
}
func statementMeta(p identity.Principal, r *http.Request) statement.Meta {
	return statement.Meta{Scope: statementPrincipal(p), IdempotencyKey: r.Header.Get("Idempotency-Key"), CorrelationID: middleware.CorrelationID(r.Context())}
}
func statementQuery(r *http.Request, allowCursor bool) (string, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", statement.ErrInvalid
	}
	for key, values := range query {
		if !allowCursor || key != "cursor" || len(values) != 1 || values[0] == "" || len(values[0]) > 512 {
			return "", statement.ErrInvalid
		}
	}
	return query.Get("cursor"), nil
}
func readStatementBody(w http.ResponseWriter, r *http.Request, out any, max int64) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &httptransport.PublicError{Status: 415, Code: "unsupported_media_type", Message: "Use application/json."}
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(out); err != nil {
		return statement.ErrInvalid
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return statement.ErrInvalid
	}
	return nil
}
func writeStatementJSON(w http.ResponseWriter, r *http.Request, value any, err error) {
	if err != nil {
		writeStatementError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}
func writeStatementReceipt(w http.ResponseWriter, r *http.Request, value statement.Receipt, err error) {
	if err == nil && value.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeStatementJSON(w, r, value, err)
}
func writeStatementError(w http.ResponseWriter, r *http.Request, err error) {
	var public *httptransport.PublicError
	if errors.As(err, &public) {
		httptransport.WriteError(w, r, public)
		return
	}
	status, code, message := 503, "statement_unavailable", "Statement evidence is temporarily unavailable. Keep the original command identity."
	switch {
	case errors.Is(err, statement.ErrInvalid), errors.Is(err, statement.ErrInvalidStatement):
		status, code, message = 400, "statement_validation_failed", "The statement command is invalid. Check the documented format."
	case errors.Is(err, statement.ErrConflict):
		status, code, message = 409, "statement_conflict", "The original intent or review version conflicts. Read current evidence before another decision."
	case errors.Is(err, statement.ErrNotFound):
		status, code, message = 404, "not_found", "Statement evidence was not found."
	case errors.Is(err, statement.ErrForbidden):
		status, code, message = 403, "forbidden", "You cannot access this statement evidence."
	case errors.Is(err, statement.ErrLimit):
		status, code, message = 422, "statement_limit", "The bounded import or snapshot limit was reached. Use a smaller supported source period."
	}
	var invalid *statement.ValidationError
	if errors.As(err, &invalid) {
		message = invalid.Error()
	}
	httptransport.WriteError(w, r, &httptransport.PublicError{Status: status, Code: code, Message: message})
}
func (h *StatementReconciliationHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, false, "workspace")
	if !ok {
		return
	}
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.Workspace(r.Context(), statementPrincipal(p))
	writeStatementJSON(w, r, value, err)
}
func (h *StatementReconciliationHandler) CreateMapping(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, true, "mapping")
	if !ok {
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
		SourceRef string `json:"source_account_ref"`
	}
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return
	}
	if err := readStatementBody(w, r, &body, 2048); err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.CreateMapping(r.Context(), statement.MappingCommand{Meta: statementMeta(p, r), AccountID: body.AccountID, SourceAccountRef: body.SourceRef})
	writeStatementReceipt(w, r, value, err)
}
func (h *StatementReconciliationHandler) Preview(w http.ResponseWriter, r *http.Request) {
	_, ok := h.authorize(w, r, true, "preview")
	if !ok {
		return
	}
	var body struct {
		Source []byte `json:"source_base64"`
	}
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return
	}
	if err := readStatementBody(w, r, &body, MaxStatementRequestBytes); err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.Preview(body.Source)
	writeStatementJSON(w, r, value, err)
}
func (h *StatementReconciliationHandler) Import(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, true, "import")
	if !ok {
		return
	}
	var body struct {
		MappingID string `json:"mapping_id"`
		Source    []byte `json:"source_base64"`
	}
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return
	}
	if err := readStatementBody(w, r, &body, MaxStatementRequestBytes); err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.Import(r.Context(), statement.ImportCommand{Meta: statementMeta(p, r), MappingID: body.MappingID, Source: body.Source})
	writeStatementReceipt(w, r, value, err)
}
func (h *StatementReconciliationHandler) Run(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, true, "run")
	if !ok {
		return
	}
	var body struct {
		ImportID string `json:"import_id"`
	}
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return
	}
	if err := readStatementBody(w, r, &body, 2048); err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.Run(r.Context(), statement.RunCommand{Meta: statementMeta(p, r), ImportID: body.ImportID})
	writeStatementReceipt(w, r, value, err)
}
func (h *StatementReconciliationHandler) RunDetail(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, false, "detail")
	if !ok {
		return
	}
	cursor, err := statementQuery(r, true)
	if err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.Page(r.Context(), statementPrincipal(p), r.PathValue("runID"), cursor)
	writeStatementJSON(w, r, value, err)
}
func (h *StatementReconciliationHandler) Review(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, true, "review")
	if !ok {
		return
	}
	var body statement.ReviewInput
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return
	}
	if err := readStatementBody(w, r, &body, 8192); err != nil {
		writeStatementError(w, r, err)
		return
	}
	value, err := h.service.Review(r.Context(), statement.ReviewCommand{Meta: statementMeta(p, r), RunID: r.PathValue("runID"), Decision: body})
	writeStatementReceipt(w, r, value, err)
}
func (h *StatementReconciliationHandler) authorizeExport(w http.ResponseWriter, r *http.Request, operation string) (identity.Principal, bool) {
	p, ok := h.authorize(w, r, false, operation)
	if !ok {
		return p, false
	}
	if identity.RequireScope(p, "exports:read") != nil {
		writeScopeDenial(w, r, h.audit, p, "exports:read")
		return p, false
	}
	if _, err := statementQuery(r, false); err != nil {
		writeStatementError(w, r, err)
		return p, false
	}
	return p, true
}
func (h *StatementReconciliationHandler) recordExport(w http.ResponseWriter, r *http.Request, p identity.Principal, id, operation string) bool {
	if h.audit == nil {
		writeStatementError(w, r, statement.ErrUnavailable)
		return false
	}
	err := h.audit.Record(r.Context(), db.AuditEvent{TenantID: p.TenantID, ActorSubjectID: p.SubjectID, EventType: "statement." + operation, TargetType: "statement", TargetID: id, Outcome: "succeeded", CorrelationID: middleware.CorrelationID(r.Context()), Metadata: map[string]string{"operation": operation}})
	if err != nil {
		writeStatementError(w, r, err)
		return false
	}
	w.Header().Set("X-Audit-Reference", middleware.CorrelationID(r.Context()))
	return true
}
func writeStatementCSV(w http.ResponseWriter, id string, data []byte) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="statement-`+id+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
func (h *StatementReconciliationHandler) Source(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorizeExport(w, r, "source_exported")
	if !ok {
		return
	}
	value, err := h.service.Source(r.Context(), statementPrincipal(p), r.PathValue("importID"))
	if err != nil {
		writeStatementError(w, r, err)
		return
	}
	if !h.recordExport(w, r, p, value.ID, "source_exported") {
		return
	}
	w.Header().Set("X-Statement-SHA256", value.SHA256)
	writeStatementCSV(w, value.ID, value.Bytes)
}
func (h *StatementReconciliationHandler) Export(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorizeExport(w, r, "comparison_exported")
	if !ok {
		return
	}
	id := r.PathValue("runID")
	value, err := h.service.Export(r.Context(), statementPrincipal(p), id)
	if err != nil {
		writeStatementError(w, r, err)
		return
	}
	if !h.recordExport(w, r, p, id, "comparison_exported") {
		return
	}
	writeStatementCSV(w, id, value)
}
