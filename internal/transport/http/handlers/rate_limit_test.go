package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type denyingLimiter struct{}

func (denyingLimiter) Consume(context.Context, string, string, string, int, time.Duration) (db.RateLimitDecision, error) {
	return db.RateLimitDecision{RetryAfter: 12 * time.Second}, nil
}

type recordedRateCall struct {
	tenantID, principalID, route string
	limit                        int
	window                       time.Duration
}

type recordingLimiter struct{ calls []recordedRateCall }

func (r *recordingLimiter) Consume(_ context.Context, tenantID, principalID, route string, limit int, window time.Duration) (db.RateLimitDecision, error) {
	r.calls = append(r.calls, recordedRateCall{tenantID: tenantID, principalID: principalID, route: route, limit: limit, window: window})
	return db.RateLimitDecision{Allowed: true}, nil
}

func TestRateLimitReturnsStableEnvelopeAndRetryAfter(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/me/accounts", nil)
	writer := httptest.NewRecorder()
	allowed := enforceRateLimit(writer, request, denyingLimiter{}, identity.Principal{TenantID: "tenant", SubjectID: "subject"}, "accounts:list", 1, false)
	if allowed {
		t.Fatal("denied rate decision was allowed")
	}
	if writer.Code != 429 || writer.Header().Get("Retry-After") != "12" || !strings.Contains(writer.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("response code=%d headers=%v body=%s", writer.Code, writer.Header(), writer.Body.String())
	}
}

func TestTenantCapacityUsesSharedSecondAndMinuteWindows(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/transfers", nil)
	writer := httptest.NewRecorder()
	limiter := &recordingLimiter{}
	principal := identity.Principal{TenantID: "tenant", SubjectID: "actor"}
	if !enforceTenantCapacity(writer, request, limiter, principal, "transfers:create", 30) {
		t.Fatalf("allowed tenant capacity was denied: %s", writer.Body.String())
	}
	if len(limiter.calls) != 2 {
		t.Fatalf("capacity calls=%d, want 2", len(limiter.calls))
	}
	want := []recordedRateCall{
		{tenantID: "tenant", principalID: tenantCapacityPrincipal, route: "transfers:create:capacity:second", limit: 30, window: time.Second},
		{tenantID: "tenant", principalID: tenantCapacityPrincipal, route: "transfers:create:capacity:minute", limit: 1_800, window: time.Minute},
	}
	for index := range want {
		if limiter.calls[index] != want[index] {
			t.Fatalf("capacity call %d=%+v, want %+v", index, limiter.calls[index], want[index])
		}
	}
}
