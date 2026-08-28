package funding

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	domainfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

type repositoryStub struct {
	requested RequestCommand
	demo      bool
	listCalls int
}

func (r *repositoryStub) Request(_ context.Context, command RequestCommand, _ [sha256.Size]byte) (Submission, error) {
	r.requested = command
	return Submission{Event: Event{FundingEventID: "funding-1", Status: string(domainfunding.StatusRequested)}}, nil
}
func (r *repositoryStub) Approve(_ context.Context, command DecisionCommand, demo bool) (Event, error) {
	r.demo = demo
	return Event{FundingEventID: command.FundingEventID, Status: string(domainfunding.StatusApproved), DemoPolicy: demo}, nil
}
func (r *repositoryStub) Reject(context.Context, DecisionCommand) (Event, error) {
	return Event{}, nil
}
func (r *repositoryStub) Post(context.Context, ActionCommand) (Submission, error) {
	return Submission{}, nil
}
func (r *repositoryStub) Compensate(context.Context, CompensationCommand, [sha256.Size]byte) (Submission, error) {
	return Submission{}, nil
}
func (r *repositoryStub) Get(context.Context, string, string, string) (Event, error) {
	return Event{}, nil
}
func (r *repositoryStub) List(context.Context, string, string, Query) (Page, error) {
	r.listCalls++
	return Page{}, nil
}
func (r *repositoryStub) Reconcile(context.Context, string, string, string) (Reconciliation, error) {
	return Reconciliation{}, nil
}

func TestServiceCanonicalizesRequestAndOwnsDemoPolicy(t *testing.T) {
	repository := &repositoryStub{}
	clock := func() time.Time { return time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC) }
	service, err := NewService(repository, PolicyLocalDemoSingleOperator, clock)
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New("USD", 5_000)
	if _, err = service.Request(context.Background(), RequestCommand{
		TenantID: " tenant ", ActorSubjectID: " actor ", DestinationAccountID: " destination ", Amount: amount,
		ExternalReference: " external-501 ", EvidenceReference: " evidence://501 ", IdempotencyKey: "funding-key-000000501", CorrelationID: " correlation ",
	}); err != nil {
		t.Fatal(err)
	}
	if repository.requested.TenantID != "tenant" || repository.requested.ActorSubjectID != "actor" || !repository.requested.RequestedAt.Equal(clock()) {
		t.Fatalf("request was not canonicalized: %#v", repository.requested)
	}
	if _, err = service.Approve(context.Background(), DecisionCommand{TenantID: "tenant", ActorSubjectID: "actor", FundingEventID: "funding-1", Reason: "local evidence reviewed", CorrelationID: "correlation"}); err != nil {
		t.Fatal(err)
	}
	if !repository.demo {
		t.Fatal("server-owned local demo policy was not applied")
	}
}

func TestServiceRejectsInvalidFundingBeforeRepository(t *testing.T) {
	repository := &repositoryStub{}
	service, _ := NewService(repository, PolicyProductionDualControl, nil)
	zero, _ := money.New("USD", 0)
	_, err := service.Request(context.Background(), RequestCommand{TenantID: "tenant", ActorSubjectID: "actor", Amount: zero, IdempotencyKey: "short"})
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("invalid request error=%v", err)
	}
	if repository.requested.TenantID != "" {
		t.Fatal("invalid request reached repository")
	}
}

func TestServiceRejectsUnknownPolicyMode(t *testing.T) {
	if _, err := NewService(&repositoryStub{}, PolicyMode("unsafe"), nil); err == nil {
		t.Fatal("unknown policy mode accepted")
	}
}

func TestServiceRejectsUnknownFundingStatusBeforeRepository(t *testing.T) {
	repository := &repositoryStub{}
	service, _ := NewService(repository, PolicyProductionDualControl, nil)
	if _, err := service.List(context.Background(), "tenant", "actor", Query{Status: "invented", Limit: 25}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("unknown status error=%v", err)
	}
	if repository.listCalls != 0 {
		t.Fatal("invalid query reached repository")
	}
}
