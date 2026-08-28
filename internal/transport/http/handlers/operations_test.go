package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type operationsDiagnosticRepositoryStub struct{ err error }

func (r operationsDiagnosticRepositoryStub) Facts(context.Context, string) (operations.DatabaseFacts, error) {
	return operations.DatabaseFacts{SchemaVersion: "15"}, r.err
}

type operationsProbeStub struct{ err error }

func (p operationsProbeStub) Ping(context.Context) error { return p.err }

type operationsEventRepositoryStub struct{}

func (operationsEventRepositoryStub) ListEvents(context.Context, string, string, operations.EventFilter) ([]operations.EventEvidence, string, error) {
	return []operations.EventEvidence{}, "", nil
}
func (operationsEventRepositoryStub) GetEvent(context.Context, string, string, string) (operations.EventDetail, error) {
	return operations.EventDetail{DeliveryAttempts: []operations.DeliveryEvidence{}, Timeline: []operations.EventTimelineItem{}}, nil
}

func operationsTestHandler(t *testing.T, scopes []string, roles []string, limiters ...RateLimiter) http.Handler {
	t.Helper()
	diagnostics, err := operations.NewDiagnosticService(operationsDiagnosticRepositoryStub{}, operationsProbeStub{}, operations.BuildFacts{Environment: "development"}, func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	events, err := operations.NewEventService(operationsEventRepositoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewOperationsHandler(diagnostics, events, identity.DevelopmentProvider{SubjectID: "operator", TenantID: "tenant", Scopes: scopes, Roles: roles})
	if len(limiters) > 0 {
		handler.WithRateLimiter(limiters[0], 1)
	}
	router := http.NewServeMux()
	router.HandleFunc("GET /api/local/diagnostics", handler.Diagnostics)
	router.HandleFunc("GET /api/events", handler.Events)
	router.HandleFunc("GET /api/events/{eventID}", handler.Event)
	return middleware.Correlation(router)
}

func TestOperationsReadsApplyRateLimit(t *testing.T) {
	limiter := &accountCommandTestLimiter{allow: false}
	response := httptest.NewRecorder()
	operationsTestHandler(t, []string{"events:read"}, []string{"tenant:operator"}, limiter).ServeHTTP(response, operationsRequest(http.MethodGet, "/api/events"))
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" || len(limiter.routes) != 1 || limiter.routes[0] != "events:list" {
		t.Fatalf("status=%d headers=%v routes=%v", response.Code, response.Header(), limiter.routes)
	}
}

func operationsRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer development-local-only")
	return request
}

func TestOperationsDiagnosticsIsAuthorizedNoStorePartialRead(t *testing.T) {
	response := httptest.NewRecorder()
	operationsTestHandler(t, []string{"local:read"}, []string{"tenant:operator"}).ServeHTTP(response, operationsRequest(http.MethodGet, "/api/local/diagnostics"))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	for _, forbidden := range []string{"postgres://", "password", "container", "payload"} {
		if containsInsensitive(response.Body.String(), forbidden) {
			t.Fatalf("diagnostic response exposed forbidden marker %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestOperationsRequiresScopeAndTenantRole(t *testing.T) {
	for name, testCase := range map[string]struct {
		scopes []string
		roles  []string
	}{
		"missing scope": {[]string{"events:read"}, []string{"tenant:operator"}},
		"missing role":  {[]string{"local:read"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			operationsTestHandler(t, testCase.scopes, testCase.roles).ServeHTTP(response, operationsRequest(http.MethodGet, "/api/local/diagnostics"))
			if response.Code != http.StatusForbidden || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestOperationsRoutesAreGETOnlyAndRejectUnknownFilters(t *testing.T) {
	handler := operationsTestHandler(t, []string{"events:read"}, []string{"tenant:operator"})
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, operationsRequest(http.MethodPost, "/api/events"))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d body=%s", post.Code, post.Body.String())
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, operationsRequest(http.MethodGet, "/api/events?payload=true"))
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown query status=%d body=%s", unknown.Code, unknown.Body.String())
	}
}

func containsInsensitive(value, marker string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(marker))
}
