package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type RateLimiter interface {
	Consume(context.Context, string, string, string, int, time.Duration) (db.RateLimitDecision, error)
}

type AuditRecorder interface {
	Record(context.Context, db.AuditEvent) error
}

func writeScopeDenial(writer http.ResponseWriter, request *http.Request, audit AuditRecorder, principal identity.Principal, scope string) {
	if audit != nil {
		err := audit.Record(request.Context(), db.AuditEvent{TenantID: principal.TenantID, ActorSubjectID: principal.SubjectID, EventType: "authorization.denied", TargetType: "api_scope", TargetID: scope, Outcome: "failed", CorrelationID: middleware.CorrelationID(request.Context()), Metadata: map[string]string{"reason": "missing_scope"}})
		if err != nil {
			httptransport.WriteError(writer, request, err)
			return
		}
	}
	httptransport.WriteError(writer, request, httptransport.ErrForbidden)
}

func enforceRateLimit(writer http.ResponseWriter, request *http.Request, limiter RateLimiter, principal identity.Principal, route string, limit int, failClosed bool) bool {
	if limiter == nil {
		return true
	}
	decision, err := limiter.Consume(request.Context(), principal.TenantID, principal.SubjectID, route, limit, time.Minute)
	if err != nil {
		if failClosed {
			httptransport.WriteError(writer, request, err)
			return false
		}
		return true
	}
	if decision.Allowed {
		return true
	}
	retrySeconds := int(decision.RetryAfter.Round(time.Second) / time.Second)
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	writer.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
	httptransport.WriteError(writer, request, &httptransport.PublicError{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "The request rate limit was exceeded. Retry after the indicated delay."})
	return false
}
