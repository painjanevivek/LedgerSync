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
