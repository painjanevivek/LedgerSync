package approvals

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repositoryStub struct {
	query Query
	err   error
}

func (r *repositoryStub) List(_ context.Context, _, _ string, query Query) (Page, error) {
	r.query = query
	return Page{Items: []Item{}, PageCount: 0}, r.err
}

func TestListRejectsUnauthorizedAndCrossDomainQueries(t *testing.T) {
	repository := &repositoryStub{}
	service, err := NewService(repository, func() time.Time { return time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tests := []Query{
		{Limit: 25},
		{Domain: DomainFunding, Limit: 25, CanApproveCorrections: true},
		{StatusDomain: DomainCorrection, Status: "requested", Limit: 25, CanApproveFunding: true},
	}
	for _, query := range tests {
		if _, err := service.List(context.Background(), "tenant", "actor", query); !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected forbidden for %#v, got %v", query, err)
		}
	}
}

func TestListValidatesTypedStatusesAndBounds(t *testing.T) {
	repository := &repositoryStub{}
	service, _ := NewService(repository, nil)
	tests := []Query{
		{Limit: 0, CanApproveFunding: true},
		{Limit: 25, CanApproveFunding: true, StatusDomain: DomainFunding, Status: "expired"},
		{Limit: 25, CanApproveFunding: true, CanApproveCorrections: true, Domain: DomainFunding, StatusDomain: DomainCorrection, Status: "requested"},
		{Limit: 25, CanApproveFunding: true, Age: "over_forever"},
		{Limit: 25, CanApproveFunding: true, RequestedAfter: time.Now(), RequestedBefore: time.Now().Add(-time.Hour)},
	}
	for _, query := range tests {
		if _, err := service.List(context.Background(), "tenant", "actor", query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("expected invalid query for %#v, got %v", query, err)
		}
	}
}

func TestListNormalizesAndDelegatesAuthorizedQuery(t *testing.T) {
	repository := &repositoryStub{}
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	service, _ := NewService(repository, func() time.Time { return now })
	_, err := service.List(context.Background(), " tenant ", " actor ", Query{
		Domain: DomainCorrection, StatusDomain: DomainCorrection, Status: " requested ",
		Requester: " requester ", Age: "over_24h", Limit: 25, CanApproveCorrections: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.Status != "requested" || repository.query.Requester != "requester" || !repository.query.Now.Equal(now) {
		t.Fatalf("query was not normalized: %#v", repository.query)
	}
}

func TestStepUpRecencyIsBounded(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	if !StepUpIsRecent(now.Add(-9*time.Minute), now) {
		t.Fatal("expected recent authentication to satisfy step-up")
	}
	if StepUpIsRecent(now.Add(-11*time.Minute), now) || StepUpIsRecent(now.Add(2*time.Minute), now) {
		t.Fatal("stale or future authentication must not satisfy step-up")
	}
}
