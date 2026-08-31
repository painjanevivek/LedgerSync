package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	appapprovals "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/approvals"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
)

type approvalListService interface {
	List(context.Context, string, string, appapprovals.Query) (appapprovals.Page, error)
}

type ApprovalHandler struct {
	service       approvalListService
	identity      identity.Provider
	authenticator *identity.RequestAuthenticator
	rateLimiter   RateLimiter
	readRate      int
	clock         func() time.Time
}

func NewApprovalHandler(service approvalListService, provider identity.Provider) *ApprovalHandler {
	return &ApprovalHandler{service: service, identity: provider, clock: time.Now}
}

func (h *ApprovalHandler) WithRequestAuthenticator(authenticator *identity.RequestAuthenticator) *ApprovalHandler {
	h.authenticator = authenticator
	return h
}

func (h *ApprovalHandler) WithRateLimiter(limiter RateLimiter, readPerMinute int) *ApprovalHandler {
	h.rateLimiter, h.readRate = limiter, readPerMinute
	return h
}

func (h *ApprovalHandler) List(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.service == nil || h.identity == nil {
		httptransport.WriteError(writer, request, errors.New("approval handler is not configured"))
		return
	}
	principal, err := h.authenticate(request)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrUnauthorized)
		return
	}
	canFunding := principal.HasScope("funding:approve")
	canCorrections := principal.HasScope("corrections:approve")
	if !canFunding && !canCorrections {
		httptransport.WriteError(writer, request, httptransport.ErrForbidden)
		return
	}
	if !enforceRateLimit(writer, request, h.rateLimiter, principal, "approvals:list", h.readRate, false) {
		return
	}
	query, err := parseApprovalQuery(request, principal, h.clock().UTC(), canFunding, canCorrections)
	if err != nil {
		httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		return
	}
	page, err := h.service.List(request.Context(), principal.TenantID, principal.SubjectID, query)
	if err != nil {
		switch {
		case errors.Is(err, appapprovals.ErrInvalidQuery):
			httptransport.WriteError(writer, request, httptransport.ErrBadRequest)
		case errors.Is(err, appapprovals.ErrForbidden):
			httptransport.WriteError(writer, request, httptransport.ErrForbidden)
		default:
			httptransport.WriteError(writer, request, &httptransport.PublicError{
				Status:  http.StatusServiceUnavailable,
				Code:    "temporary_unavailable",
				Message: "Approval evidence is temporarily unavailable.",
			})
		}
		return
	}
	writeFundingJSON(writer, http.StatusOK, page)
}

func (h *ApprovalHandler) authenticate(request *http.Request) (identity.Principal, error) {
	assertion := request.Header.Get("X-LedgerSync-Actor-Assertion")
	if h.authenticator != nil {
		return h.authenticator.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")), assertion)
	}
	if assertion != "" {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return h.identity.Authenticate(request.Context(), bearerToken(request.Header.Get("Authorization")))
}

func parseApprovalQuery(request *http.Request, principal identity.Principal, now time.Time, canFunding, canCorrections bool) (appapprovals.Query, error) {
	values := request.URL.Query()
	allowed := map[string]struct{}{
		"domain": {}, "status": {}, "requester": {}, "age": {},
		"requested_after": {}, "requested_before": {}, "actionable_by_me": {},
		"cursor": {}, "limit": {},
	}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return appapprovals.Query{}, errors.New("unknown or repeated approval filter")
		}
	}
	domain := appapprovals.Domain(strings.TrimSpace(values.Get("domain")))
	if domain == "all" {
		domain = ""
	}
	statusDomain, status, err := parseApprovalStatus(values.Get("status"))
	if err != nil {
		return appapprovals.Query{}, err
	}
	after, err := parseApprovalDate(values.Get("requested_after"), false)
	if err != nil {
		return appapprovals.Query{}, err
	}
	before, err := parseApprovalDate(values.Get("requested_before"), true)
	if err != nil {
		return appapprovals.Query{}, err
	}
	limit := 25
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return appapprovals.Query{}, err
		}
	}
	actionable := false
	if raw := strings.TrimSpace(values.Get("actionable_by_me")); raw != "" {
		actionable, err = strconv.ParseBool(raw)
		if err != nil {
			return appapprovals.Query{}, err
		}
	}
	return appapprovals.Query{
		Domain: domain, StatusDomain: statusDomain, Status: status,
		Requester: values.Get("requester"), Age: values.Get("age"),
		RequestedAfter: after, RequestedBefore: before, ActionableOnly: actionable,
		Cursor: values.Get("cursor"), Limit: limit,
		CanApproveFunding: canFunding, CanApproveCorrections: canCorrections,
		StepUpAuthenticatedAt: principal.AuthenticatedAt, Now: now,
	}, nil
}

func parseApprovalStatus(value string) (appapprovals.Domain, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", nil
	}
	prefix, status, ok := strings.Cut(value, ":")
	if !ok || status == "" {
		return "", "", errors.New("approval status must be domain-qualified")
	}
	return appapprovals.Domain(prefix), status, nil
}

func parseApprovalDate(value string, endOfDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return parsed.UTC(), nil
}
