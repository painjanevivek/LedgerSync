package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/guidance"
	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

const handlerGuidanceTransferID = "70000000-0000-4000-8000-000000000001"

type handlerGuidanceRepository struct {
	facts guidance.TransferFacts
	err   error
}

func (s handlerGuidanceRepository) Orientation(context.Context, string, string) (guidance.OrientationFacts, error) {
	return guidance.OrientationFacts{}, s.err
}

func (s handlerGuidanceRepository) ExplainTransfer(context.Context, string, string, string) (guidance.TransferFacts, error) {
	return s.facts, s.err
}

type handlerGuidanceRecovery struct{}

func (handlerGuidanceRecovery) Snapshot(context.Context) (apprecovery.ManifestSnapshot, error) {
	return apprecovery.ManifestSnapshot{}, nil
}

func guidanceTestHandler(t *testing.T, scopes, roles []string, repository handlerGuidanceRepository) http.Handler {
	t.Helper()
	service, err := guidance.NewService(repository, handlerGuidanceRecovery{}, func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	handler := NewGuidanceHandler(service, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: scopes, Roles: roles})
	router := http.NewServeMux()
	router.HandleFunc("GET /api/local/orientation", handler.Orientation)
	router.HandleFunc("GET /api/transfers/{transferID}/explainability", handler.ExplainTransfer)
	return middleware.Correlation(router)
}

func guidanceRequest(target string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", "Bearer development-local-only")
	return request
}

func TestGuidanceRoutesAreNoStoreBoundedEvidenceReads(t *testing.T) {
	when := time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)
	facts := guidance.TransferFacts{TransferID: handlerGuidanceTransferID, Transfer: guidance.EvidenceLink{Items: []guidance.EvidenceItem{{EvidenceType: "transfer", EvidenceID: handlerGuidanceTransferID, Status: "posted", OccurredAt: &when}}}}
	handler := guidanceTestHandler(t, []string{"local:read", "explainability:read", "transfers:read", "events:read", "reconciliation:read"}, []string{"tenant:operator"}, handlerGuidanceRepository{facts: facts})
	for _, target := range []string{"/api/local/orientation", "/api/transfers/" + handlerGuidanceTransferID + "/explainability"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, guidanceRequest(target))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s status=%d headers=%v body=%s", target, response.Code, response.Header(), response.Body.String())
		}
	}
}

func TestExplainabilityRequiresAllEvidenceScopesAndUsesNonDisclosingNotFound(t *testing.T) {
	for _, scopes := range [][]string{
		{"explainability:read", "transfers:read", "events:read"},
		{"transfers:read", "events:read", "reconciliation:read"},
	} {
		response := httptest.NewRecorder()
		guidanceTestHandler(t, scopes, nil, handlerGuidanceRepository{}).ServeHTTP(response, guidanceRequest("/api/transfers/"+handlerGuidanceTransferID+"/explainability"))
		if response.Code != http.StatusForbidden {
			t.Fatalf("scopes=%v status=%d body=%s", scopes, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	allScopes := []string{"explainability:read", "transfers:read", "events:read", "reconciliation:read"}
	guidanceTestHandler(t, allScopes, nil, handlerGuidanceRepository{err: guidance.ErrTransferNotFound}).ServeHTTP(response, guidanceRequest("/api/transfers/"+handlerGuidanceTransferID+"/explainability"))
	if response.Code != http.StatusNotFound || strings.Contains(strings.ToLower(response.Body.String()), "tenant") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOrientationRequiresOperatorAndBothRoutesRejectQueries(t *testing.T) {
	withoutRole := httptest.NewRecorder()
	guidanceTestHandler(t, []string{"local:read"}, nil, handlerGuidanceRepository{}).ServeHTTP(withoutRole, guidanceRequest("/api/local/orientation"))
	if withoutRole.Code != http.StatusForbidden {
		t.Fatalf("orientation role status=%d body=%s", withoutRole.Code, withoutRole.Body.String())
	}

	response := httptest.NewRecorder()
	guidanceTestHandler(t, []string{"local:read"}, []string{"tenant:operator"}, handlerGuidanceRepository{}).ServeHTTP(response, guidanceRequest("/api/local/orientation?completed=true"))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown query status=%d body=%s", response.Code, response.Body.String())
	}
}
