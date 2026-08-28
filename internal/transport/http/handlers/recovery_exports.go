package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	appexports "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/exports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const (
	defaultExportRows = 10_000
	maxExportRows     = 10_000
	exportDeadline    = 10 * time.Second
	recoveryDeadline  = 5 * time.Second
	delayedCSVBytes   = 32 * 1024
)

var canonicalExportUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type RecoveryExportHandler struct {
	manifests     *apprecovery.ManifestService
	exports       *appexports.Service
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	rateLimit     int
	audit         AuditRecorder
	clock         func() time.Time
}

func NewRecoveryExportHandler(manifests *apprecovery.ManifestService, exportService *appexports.Service, provider identity.Provider) *RecoveryExportHandler {
	return &RecoveryExportHandler{manifests: manifests, exports: exportService, identity: provider, clock: time.Now}
}

func (h *RecoveryExportHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *RecoveryExportHandler {
	h.authenticator = authenticator
	return h
}

func (h *RecoveryExportHandler) WithRateLimiter(limiter RateLimiter, limit int) *RecoveryExportHandler {
	h.rateLimiter, h.rateLimit = limiter, limit
	return h
}

func (h *RecoveryExportHandler) WithAuditRecorder(audit AuditRecorder) *RecoveryExportHandler {
	h.audit = audit
	return h
}

func (h *RecoveryExportHandler) WithClock(clock func() time.Time) *RecoveryExportHandler {
	if clock != nil {
		h.clock = clock
	}
	return h
}

func (h *RecoveryExportHandler) Manifests(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, []string{"recovery:read"}, true, "recovery:manifests")
	if !ok {
		return
	}
	if h.manifests == nil || !onlyQueryParameters(request) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), recoveryDeadline)
	defer cancel()
	snapshot, err := h.manifests.Snapshot(ctx)
	if err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "recovery_evidence_unavailable", Message: "Sanitized recovery evidence is temporarily unavailable."})
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(writer).Encode(snapshot)
	_ = principal
}

func (h *RecoveryExportHandler) TransfersCSV(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, []string{"exports:read", "transfers:read"}, true, "exports:transfers")
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "accountId", "status", "q", "from", "to", "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	filter, rows, valid := transferExportQuery(request)
	if !valid {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	fingerprint := appexports.FilterFingerprint(filter.AccountID, filter.Status, filter.Query, utcQuery(filter.From), utcQuery(filter.To), strconv.Itoa(rows))
	h.streamCSV(writer, request, principal, "transfers", "transfers", rows, fingerprint, func(ctx context.Context, destination *delayedCSVWriter) (appexports.Result, error) {
		return h.exports.StreamTransfers(ctx, principal.TenantID, filter, rows, destination)
	})
}

func (h *RecoveryExportHandler) AccountLedgerCSV(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, []string{"exports:read", "transactions:read"}, false, "exports:account-ledger")
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	rows, valid := boundedIntegerQuery(request, "limit", defaultExportRows, maxExportRows)
	accountID := strings.TrimSpace(request.PathValue("accountID"))
	if !valid || !canonicalExportUUID.MatchString(accountID) {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	fingerprint := appexports.FilterFingerprint("account", accountID, strconv.Itoa(rows))
	h.streamCSV(writer, request, principal, "account-ledger", "account-ledger", rows, fingerprint, func(ctx context.Context, destination *delayedCSVWriter) (appexports.Result, error) {
		return h.exports.StreamAccountLedger(ctx, principal.TenantID, principal.SubjectID, accountID, rows, destination)
	})
}

func (h *RecoveryExportHandler) ReconciliationCSV(writer http.ResponseWriter, request *http.Request) {
	principal, ok := h.authorize(writer, request, []string{"exports:read", "reconciliation:read"}, true, "exports:reconciliation")
	if !ok {
		return
	}
	if !onlyQueryParameters(request, "runId", "status", "from", "to", "limit") {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	filter, rows, valid := reconciliationExportQuery(request)
	if !valid {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	fingerprint := appexports.FilterFingerprint(filter.RunID, filter.Status, utcQuery(filter.From), utcQuery(filter.To), strconv.Itoa(rows))
	h.streamCSV(writer, request, principal, "reconciliation", "reconciliation", rows, fingerprint, func(ctx context.Context, destination *delayedCSVWriter) (appexports.Result, error) {
		return h.exports.StreamReconciliation(ctx, principal.TenantID, filter, rows, destination)
	})
}

type exportStream func(context.Context, *delayedCSVWriter) (appexports.Result, error)

func (h *RecoveryExportHandler) streamCSV(writer http.ResponseWriter, request *http.Request, principal identity.Principal, exportType, filenameFamily string, rowLimit int, fingerprint string, stream exportStream) {
	if h.exports == nil || h.audit == nil {
		httptransport.WriteError(writer, request, errors.New("export handler is not configured"))
		return
	}
	correlationID := middleware.CorrelationID(request.Context())
	if err := h.recordExportAudit(request.Context(), principal, exportType, correlationID, "requested", rowLimit, 0, false, fingerprint); err != nil {
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "export_unavailable", Message: "The export could not establish required audit evidence."})
		return
	}
	filename := "ledgersync-" + filenameFamily + "-" + h.clock().UTC().Format("20060102T150405Z") + "-v" + appexports.SchemaVersion + ".csv"
	destination := newDelayedCSVWriter(writer, filename, rowLimit)
	ctx, cancel := context.WithTimeout(request.Context(), exportDeadline)
	defer cancel()
	result, err := stream(ctx, destination)
	if err != nil {
		destination.fail()
		_ = h.recordExportAuditDetached(request.Context(), principal, exportType, correlationID, "failed", rowLimit, result.Rows, result.Truncated, fingerprint)
		if !destination.committed {
			writeExportError(writer, request, err)
		}
		return
	}
	if err = h.recordExportAuditDetached(request.Context(), principal, exportType, correlationID, "completed", rowLimit, result.Rows, result.Truncated, fingerprint); err != nil {
		destination.fail()
		if !destination.committed {
			httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "export_unavailable", Message: "The export could not finalize required audit evidence."})
		}
		return
	}
	destination.complete(result)
}

func (h *RecoveryExportHandler) authorize(writer http.ResponseWriter, request *http.Request, scopes []string, requireOperator bool, route string) (identity.Principal, bool) {
	if request.Method != http.MethodGet {
		writeGETOnly(writer, request)
		return identity.Principal{}, false
	}
	if h == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("recovery/export handler is not configured"))
		return identity.Principal{}, false
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return identity.Principal{}, false
	}
	for _, scope := range scopes {
		if identity.RequireScope(principal, scope) != nil {
			writeScopeDenial(writer, request, h.audit, principal, scope)
			return identity.Principal{}, false
		}
	}
	if requireOperator && !principal.HasRole("tenant:operator") && !principal.HasRole("tenant:admin") {
		writeScopeDenial(writer, request, h.audit, principal, "tenant:export")
		return identity.Principal{}, false
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, route, h.rateLimit, false) {
		return identity.Principal{}, false
	}
	return principal, true
}

func (h *RecoveryExportHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func (h *RecoveryExportHandler) recordExportAudit(ctx context.Context, principal identity.Principal, exportType, correlationID, outcome string, limit, count int, truncated bool, fingerprint string) error {
	if h.audit == nil {
		return errors.New("audit recorder is required")
	}
	auditOutcome := "failed"
	switch outcome {
	case "requested":
		auditOutcome = "allowed"
	case "completed":
		auditOutcome = "succeeded"
	case "failed":
		// Keep the append-only audit outcome inside the database constraint while
		// retaining the more precise lifecycle state in the event type.
	default:
		return errors.New("unsupported export audit outcome")
	}
	return h.audit.Record(ctx, db.AuditEvent{
		TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, EventType: "export." + outcome, TargetType: "evidence_export", TargetID: correlationID, Outcome: auditOutcome, CorrelationID: correlationID,
		Metadata: map[string]string{"export_type": exportType, "filter_fingerprint": fingerprint, "schema_version": appexports.SchemaVersion, "row_limit": strconv.Itoa(limit), "row_count": strconv.Itoa(count), "limit_reached": strconv.FormatBool(truncated)}, OccurredAt: h.clock().UTC(),
	})
}

func (h *RecoveryExportHandler) recordExportAuditDetached(parent context.Context, principal identity.Principal, exportType, correlationID, outcome string, limit, count int, truncated bool, fingerprint string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 2*time.Second)
	defer cancel()
	return h.recordExportAudit(ctx, principal, exportType, correlationID, outcome, limit, count, truncated, fingerprint)
}

func transferExportQuery(request *http.Request) (investigation.TransferFilter, int, bool) {
	query := request.URL.Query()
	rows, ok := boundedIntegerQuery(request, "limit", defaultExportRows, maxExportRows)
	filter := investigation.TransferFilter{AccountID: strings.TrimSpace(query.Get("accountId")), Status: strings.TrimSpace(query.Get("status")), Query: strings.TrimSpace(query.Get("q"))}
	if !ok || (filter.AccountID != "" && !canonicalExportUUID.MatchString(filter.AccountID)) || (filter.Status != "" && filter.Status != "pending" && filter.Status != "posted" && filter.Status != "rejected") || !boundedQueryText(filter.Query, 256) {
		return filter, 0, false
	}
	filter.From, filter.To, ok = timeRange(query.Get("from"), query.Get("to"))
	return filter, rows, ok
}

func reconciliationExportQuery(request *http.Request) (appexports.ReconciliationFilter, int, bool) {
	query := request.URL.Query()
	rows, ok := boundedIntegerQuery(request, "limit", defaultExportRows, maxExportRows)
	filter := appexports.ReconciliationFilter{RunID: strings.TrimSpace(query.Get("runId")), Status: strings.TrimSpace(query.Get("status"))}
	if !ok || (filter.RunID != "" && !canonicalExportUUID.MatchString(filter.RunID)) || (filter.Status != "" && filter.Status != "matched" && filter.Status != "mismatch" && filter.Status != "failed") {
		return filter, 0, false
	}
	filter.From, filter.To, ok = timeRange(query.Get("from"), query.Get("to"))
	return filter, rows, ok
}

func boundedIntegerQuery(request *http.Request, name string, fallback, maximum int) (int, bool) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= 1 && value <= maximum
}

func timeRange(fromRaw, toRaw string) (time.Time, time.Time, bool) {
	var from, to time.Time
	var err error
	if strings.TrimSpace(fromRaw) != "" {
		from, err = time.Parse(time.RFC3339, fromRaw)
	}
	if err == nil && strings.TrimSpace(toRaw) != "" {
		to, err = time.Parse(time.RFC3339, toRaw)
	}
	return from.UTC(), to.UTC(), err == nil && (from.IsZero() || to.IsZero() || !from.After(to))
}

func boundedQueryText(value string, maximum int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func utcQuery(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func writeExportError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusGatewayTimeout, Code: "export_timeout", Message: "The export exceeded its bounded generation window."})
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, appexports.ErrInvalidRequest):
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
	case errors.Is(err, transactions.ErrHistoryNotFound):
		httptransport.WriteError(writer, request, httptransport.ErrNotFound)
	default:
		httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusServiceUnavailable, Code: "export_unavailable", Message: "Authorized export evidence is temporarily unavailable."})
	}
}

type delayedCSVWriter struct {
	response  http.ResponseWriter
	buffer    bytes.Buffer
	filename  string
	rowLimit  int
	committed bool
}

func newDelayedCSVWriter(response http.ResponseWriter, filename string, rowLimit int) *delayedCSVWriter {
	return &delayedCSVWriter{response: response, filename: filename, rowLimit: rowLimit}
}

func (w *delayedCSVWriter) Write(content []byte) (int, error) {
	if !w.committed && w.buffer.Len()+len(content) <= delayedCSVBytes {
		return w.buffer.Write(content)
	}
	if err := w.commit(); err != nil {
		return 0, err
	}
	return w.response.Write(content)
}

func (w *delayedCSVWriter) commit() error {
	if w.committed {
		return nil
	}
	header := w.response.Header()
	header.Set("Content-Type", "text/csv; charset=utf-8")
	header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, w.filename))
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-LedgerSync-Export-Schema", appexports.SchemaVersion)
	header.Set("X-LedgerSync-Export-Row-Limit", strconv.Itoa(w.rowLimit))
	header.Set("Trailer", "X-LedgerSync-Export-Status, X-LedgerSync-Export-Rows, X-LedgerSync-Export-Limit-Reached")
	w.response.WriteHeader(http.StatusOK)
	w.committed = true
	if w.buffer.Len() == 0 {
		return nil
	}
	_, err := w.response.Write(w.buffer.Bytes())
	w.buffer.Reset()
	return err
}

func (w *delayedCSVWriter) complete(result appexports.Result) {
	if err := w.commit(); err != nil {
		w.fail()
		return
	}
	w.response.Header().Set("X-LedgerSync-Export-Status", "complete")
	w.response.Header().Set("X-LedgerSync-Export-Rows", strconv.Itoa(result.Rows))
	w.response.Header().Set("X-LedgerSync-Export-Limit-Reached", strconv.FormatBool(result.Truncated))
}

func (w *delayedCSVWriter) fail() {
	if w.committed {
		w.response.Header().Set("X-LedgerSync-Export-Status", "failed")
	}
}
