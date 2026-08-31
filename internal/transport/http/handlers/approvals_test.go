package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appapprovals "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/approvals"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type approvalServiceStub struct {
	query appapprovals.Query
	calls int
	err   error
}

func (s *approvalServiceStub) List(_ context.Context, _, _ string, query appapprovals.Query) (appapprovals.Page, error) {
	s.query, s.calls = query, s.calls+1
	if s.err != nil {
		return appapprovals.Page{}, s.err
	}
	return appapprovals.Page{Items: []appapprovals.Item{{Domain: appapprovals.DomainFunding, RecordID: "funding-1", Status: "requested"}}, PageCount: 1}, nil
}

func TestApprovalListPreservesTypedFiltersAndHonestPageCount(t *testing.T) {
	service := &approvalServiceStub{}
	handler := NewApprovalHandler(service, identity.DevelopmentProvider{
		SubjectID: "finance-1", TenantID: "tenant-1", Scopes: []string{"funding:approve", "corrections:approve"},
	})
	handler.clock = func() time.Time { return time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC) }
	request := httptest.NewRequest(http.MethodGet, "/api/approvals?domain=funding&status=funding%3Arequested&requester=operator-1&age=over_24h&requested_after=2026-08-01&requested_before=2026-08-31&actionable_by_me=true&limit=25", nil)
	request.Header.Set("Authorization", "Bearer development-local-only")
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.List)).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || service.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, service.calls, response.Body.String())
	}
	if service.query.Domain != appapprovals.DomainFunding || service.query.StatusDomain != appapprovals.DomainFunding || service.query.Status != "requested" || service.query.Requester != "operator-1" || service.query.Age != "over_24h" || !service.query.ActionableOnly || service.query.Limit != 25 {
		t.Fatalf("query=%#v", service.query)
	}
	if service.query.RequestedAfter.Format("2006-01-02") != "2026-08-01" || service.query.RequestedBefore.Format("2006-01-02") != "2026-08-31" {
		t.Fatalf("date range=%s..%s", service.query.RequestedAfter, service.query.RequestedBefore)
	}
}

func TestApprovalListReturnsSafeUnavailableContractForDependencyFailures(t *testing.T) {
	service := &approvalServiceStub{err: errors.New("database connection refused: internal-only detail")}
	handler := NewApprovalHandler(service, identity.DevelopmentProvider{
		SubjectID: "finance-1", TenantID: "tenant-1", Scopes: []string{"funding:approve"},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/approvals", nil)
	request.Header.Set("Authorization", "Bearer development-local-only")
	response := httptest.NewRecorder()
	middleware.Correlation(http.HandlerFunc(handler.List)).ServeHTTP(response, request)

	body := response.Body.String()
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(body, `"code":"temporary_unavailable"`) {
		t.Fatalf("status=%d body=%s", response.Code, body)
	}
	if strings.Contains(body, "database connection refused") {
		t.Fatalf("internal dependency detail leaked: %s", body)
	}
}

func TestApprovalListSeparatesDeniedFromInvalidQueries(t *testing.T) {
	for name, testCase := range map[string]struct {
		scopes []string
		target string
		status int
	}{
		"missing approval authority": {[]string{"funding:read"}, "/api/approvals", http.StatusForbidden},
		"unqualified status":         {[]string{"funding:approve"}, "/api/approvals?status=requested", http.StatusBadRequest},
		"unknown filter":             {[]string{"funding:approve"}, "/api/approvals?evidence=raw", http.StatusBadRequest},
		"repeated filter":            {[]string{"funding:approve"}, "/api/approvals?domain=funding&domain=correction", http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			service := &approvalServiceStub{}
			handler := NewApprovalHandler(service, identity.DevelopmentProvider{SubjectID: "finance-1", TenantID: "tenant-1", Scopes: testCase.scopes})
			request := httptest.NewRequest(http.MethodGet, testCase.target, nil)
			request.Header.Set("Authorization", "Bearer development-local-only")
			response := httptest.NewRecorder()
			middleware.Correlation(http.HandlerFunc(handler.List)).ServeHTTP(response, request)
			if response.Code != testCase.status || service.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, service.calls, response.Body.String())
			}
		})
	}
}
