package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appexports "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/exports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const exportTestAccountID = "70000000-0000-4000-8000-000000000001"

type recoveryIndexStub struct {
	snapshot apprecovery.ManifestSnapshot
	err      error
}

func (s recoveryIndexStub) Snapshot(context.Context) (apprecovery.ManifestSnapshot, error) {
	return s.snapshot, s.err
}

type exportRepositoryStub struct {
	transferErr error
	accountErr  error
}

func (s exportRepositoryStub) ListTransfers(context.Context, string, investigation.TransferFilter) ([]investigation.TransferSummary, string, error) {
	if s.transferErr != nil {
		return nil, "", s.transferErr
	}
	when := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	return []investigation.TransferSummary{{
		ID: "70000000-0000-4000-8000-000000000002", DebitAccountID: exportTestAccountID,
		CreditAccountID: "70000000-0000-4000-8000-000000000003", AmountMinor: "12550", Currency: "INR",
		FinancialStatus: "posted", DeliveryStatus: "delivered", CreatedAt: when, CompletedAt: when,
		JournalTransactionID: "70000000-0000-4000-8000-000000000004", RejectionCode: "=unsafe",
	}}, "", nil
}

func (s exportRepositoryStub) ListAccountHistory(context.Context, string, string, string, string, int) ([]transactions.Entry, string, error) {
	if s.accountErr != nil {
		return nil, "", s.accountErr
	}
	return []transactions.Entry{{TransferID: "70000000-0000-4000-8000-000000000002", Direction: "credit", Amount: "12550", Currency: "INR", Status: "posted", OccurredAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)}}, "", nil
}

func (exportRepositoryStub) ListReconciliationRuns(context.Context, string, appexports.ReconciliationFilter) ([]investigation.ReconciliationRun, string, error) {
	return nil, "", nil
}

func (exportRepositoryStub) ListReconciliationMismatches(context.Context, string, string, string, int) ([]investigation.ReconciliationMismatch, string, error) {
	return nil, "", nil
}

type exportAuditStub struct {
	events []db.AuditEvent
	err    error
}

func (s *exportAuditStub) Record(_ context.Context, event db.AuditEvent) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, event)
	return nil
}

func recoveryExportTestHandler(t *testing.T, repository exportRepositoryStub, scopes, roles []string, audit *exportAuditStub) http.Handler {
	t.Helper()
	manifestService, err := apprecovery.NewManifestService(recoveryIndexStub{snapshot: apprecovery.ManifestSnapshot{
		FormatVersion: "ledgersync-recovery-evidence-index/v1", GeneratedAtUTC: "2026-08-25T12:00:00Z",
		Retention: apprecovery.ManifestRetention{ValidBackupCount: 1, ConfiguredKeepCount: 3},
	}})
	if err != nil {
		t.Fatal(err)
	}
	exportService, err := appexports.NewService(repository, appexports.DefaultMaxRows, 250)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewRecoveryExportHandler(manifestService, exportService, identity.DevelopmentProvider{
		SubjectID: "operator", TenantID: "tenant", Scopes: scopes, Roles: roles,
	}).WithAuditRecorder(audit).WithClock(func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) })
	router := http.NewServeMux()
	router.HandleFunc("GET /api/recovery/manifests", handler.Manifests)
	router.HandleFunc("GET /api/exports/transfers.csv", handler.TransfersCSV)
	router.HandleFunc("GET /api/exports/accounts/{accountID}/transactions.csv", handler.AccountLedgerCSV)
	return middleware.Correlation(router)
}

func exportRequest(target string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", "Bearer development-local-only")
	return request
}

func TestRecoveryIndexIsExactNoStoreOperatorRead(t *testing.T) {
	handler := recoveryExportTestHandler(t, exportRepositoryStub{}, []string{"recovery:read"}, []string{"tenant:operator"}, &exportAuditStub{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, exportRequest("/api/recovery/manifests"))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	for _, required := range []string{`"format_version":"ledgersync-recovery-evidence-index/v1"`, `"latest_backup":null`, `"latest_restore":null`, `"retention"`} {
		if !strings.Contains(response.Body.String(), required) {
			t.Fatalf("recovery response missing %q: %s", required, response.Body.String())
		}
	}
	for _, forbidden := range []string{"path", "filename", "digest_value", "credential", "docker", "project"} {
		if strings.Contains(strings.ToLower(response.Body.String()), forbidden) {
			t.Fatalf("recovery response exposed forbidden field %q: %s", forbidden, response.Body.String())
		}
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, exportRequest("/api/recovery/manifests?path=backup.dump"))
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown recovery query status=%d body=%s", unknown.Code, unknown.Body.String())
	}
}

func TestTransferExportStreamsQuotedExactCSVAndAuditSummary(t *testing.T) {
	audit := &exportAuditStub{}
	handler := recoveryExportTestHandler(t, exportRepositoryStub{}, []string{"exports:read", "transfers:read"}, []string{"tenant:operator"}, audit)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, exportRequest("/api/exports/transfers.csv?status=posted&limit=10"))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/csv; charset=utf-8" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	if response.Header().Get("Content-Disposition") != `attachment; filename="ledgersync-transfers-20260825T120000Z-v2.csv"` || response.Header().Get("X-LedgerSync-Export-Schema") != "2" {
		t.Fatalf("export headers=%v", response.Header())
	}
	if !strings.Contains(response.Body.String(), `"12550","INR"`) || !strings.Contains(response.Body.String(), `"'=unsafe"`) {
		t.Fatalf("CSV lost exact money or formula neutralization: %s", response.Body.String())
	}
	if response.Header().Get("X-LedgerSync-Export-Status") != "complete" || response.Header().Get("X-LedgerSync-Export-Rows") != "1" {
		t.Fatalf("trailers=%v", response.Header())
	}
	if len(audit.events) != 2 || audit.events[0].EventType != "export.requested" || audit.events[1].EventType != "export.completed" || audit.events[1].Metadata["row_count"] != "1" {
		t.Fatalf("audit events=%+v", audit.events)
	}
	if audit.events[0].Outcome != "allowed" || audit.events[1].Outcome != "succeeded" {
		t.Fatalf("audit outcomes must satisfy the append-only database constraint: %+v", audit.events)
	}
	for _, event := range audit.events {
		encoded := strings.ToLower(event.Metadata["filter_fingerprint"] + event.Metadata["export_type"])
		if strings.Contains(encoded, "12550") || strings.Contains(encoded, "inr") {
			t.Fatalf("audit persisted row evidence: %+v", event)
		}
	}
}

func TestExportsRequireCombinedScopeRoleAndObjectAuthorization(t *testing.T) {
	for name, testCase := range map[string]struct {
		scopes []string
		roles  []string
	}{
		"missing underlying read scope": {[]string{"exports:read"}, []string{"tenant:operator"}},
		"missing operator role":         {[]string{"exports:read", "transfers:read"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			recorder := &exportAuditStub{}
			recoveryExportTestHandler(t, exportRepositoryStub{}, testCase.scopes, testCase.roles, recorder).ServeHTTP(response, exportRequest("/api/exports/transfers.csv"))
			if response.Code != http.StatusForbidden || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
		})
	}

	response := httptest.NewRecorder()
	handler := recoveryExportTestHandler(t, exportRepositoryStub{accountErr: transactions.ErrHistoryNotFound}, []string{"exports:read", "transactions:read"}, nil, &exportAuditStub{})
	handler.ServeHTTP(response, exportRequest("/api/exports/accounts/"+exportTestAccountID+"/transactions.csv"))
	if response.Code != http.StatusNotFound || strings.Contains(strings.ToLower(response.Body.String()), "owner") {
		t.Fatalf("object denial status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestExportRejectsUnknownFiltersAndFailsClosedOnAuditOrEvidence(t *testing.T) {
	for name, testCase := range map[string]struct {
		handler http.Handler
		target  string
	}{
		"unknown filter": {
			recoveryExportTestHandler(t, exportRepositoryStub{}, []string{"exports:read", "transfers:read"}, []string{"tenant:operator"}, &exportAuditStub{}),
			"/api/exports/transfers.csv?cursor=arbitrary",
		},
		"audit unavailable": {
			recoveryExportTestHandler(t, exportRepositoryStub{}, []string{"exports:read", "transfers:read"}, []string{"tenant:operator"}, &exportAuditStub{err: errors.New("audit unavailable")}),
			"/api/exports/transfers.csv",
		},
		"evidence unavailable": {
			recoveryExportTestHandler(t, exportRepositoryStub{transferErr: errors.New("database unavailable")}, []string{"exports:read", "transfers:read"}, []string{"tenant:operator"}, &exportAuditStub{}),
			"/api/exports/transfers.csv",
		},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			testCase.handler.ServeHTTP(response, exportRequest(testCase.target))
			want := http.StatusBadRequest
			if name != "unknown filter" {
				want = http.StatusServiceUnavailable
			}
			if response.Code != want || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
		})
	}
}
