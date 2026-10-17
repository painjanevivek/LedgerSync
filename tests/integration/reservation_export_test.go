package integration_test

import (
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

func checkReservationExport(t *testing.T) {
	admin, api, service := reservationFixture(t)
	created, err := service.Create(context.Background(), reservationCreate("500", 3600))
	if err != nil {
		t.Fatal(err)
	}
	before := statementFinancialHash(t, admin)
	audit, err := db.NewAuditRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	provider := identity.DevelopmentProvider{TenantID: testTenantID, SubjectID: testActorID, Scopes: []string{"transfers:read", "exports:read"}}
	call := func(p identity.DevelopmentProvider, query string) *httptest.ResponseRecorder {
		h := handlers.NewReservationHandler(service, p).WithAuditRecorder(audit)
		r := httptest.NewRequest(http.MethodGet, "/api/reservations/export.csv"+query, nil)
		r.Header.Set("Authorization", "Bearer development-local-only")
		w := httptest.NewRecorder()
		middleware.Correlation(http.HandlerFunc(h.Export)).ServeHTTP(w, r)
		return w
	}
	result := call(provider, "")
	if result.Code != 200 || result.Header().Get("X-Audit-Reference") == "" {
		t.Fatalf("audited export status %d", result.Code)
	}
	rows, err := csv.NewReader(strings.NewReader(result.Body.String())).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][2] != created.ID || rows[1][5] != "500" || rows[1][3] != "active" {
		t.Fatal("active obligation export differs", err)
	}
	denied := provider
	denied.Scopes = []string{"transfers:read"}
	if result := call(denied, ""); result.Code != 403 {
		t.Fatal("export scope not enforced", result.Code)
	}
	other := provider
	other.SubjectID = "synthetic-other-actor"
	if result := call(other, ""); result.Code != 200 || strings.Contains(result.Body.String(), created.ID) {
		t.Fatal("another actor read the obligation")
	}
	if result := call(provider, "?tenant_id="+testTenantID); result.Code != 400 {
		t.Fatal("export scope query accepted", result.Code)
	}
	if _, err := admin.Exec(`CREATE FUNCTION r16_export_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'synthetic audit failure';END$$;CREATE TRIGGER r16_export_fail_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION r16_export_fail_audit()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP TRIGGER IF EXISTS r16_export_fail_audit ON audit_events;DROP FUNCTION IF EXISTS r16_export_fail_audit()`); err != nil {
			t.Error(err)
		}
	})
	result = call(provider, "")
	if result.Code != 503 || strings.Contains(result.Body.String(), created.ID) || result.Header().Get("Content-Type") == "text/csv; charset=utf-8" {
		t.Fatal("CSV disclosed before a successful audit")
	}
	if before != statementFinancialHash(t, admin) {
		t.Fatal("obligation export changed money")
	}
}
