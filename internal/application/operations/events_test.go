package operations

import (
	"context"
	"errors"
	"testing"
)

type eventRepositoryStub struct {
	filter  EventFilter
	actorID string
}

func (r *eventRepositoryStub) ListEvents(_ context.Context, _ string, actorID string, filter EventFilter) ([]EventEvidence, string, error) {
	r.actorID, r.filter = actorID, filter
	return []EventEvidence{}, "", nil
}
func (r *eventRepositoryStub) GetEvent(context.Context, string, string, string) (EventDetail, error) {
	return EventDetail{}, nil
}

func TestEventServiceValidatesFiltersBeforeRepository(t *testing.T) {
	repository := &eventRepositoryStub{}
	service, err := NewEventService(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.List(context.Background(), "tenant", "actor", EventFilter{State: "published", RelatedID: "00000000-0000-0000-0000-000000000010", Limit: 25}); err != nil {
		t.Fatal(err)
	}
	if repository.actorID != "actor" || repository.filter.State != "published" {
		t.Fatalf("normalized filter not delegated: %#v", repository.filter)
	}
	if _, _, err := service.List(context.Background(), "tenant", "actor", EventFilter{State: "healthy", Limit: 25}); !errors.Is(err, ErrInvalidEventEvidence) {
		t.Fatalf("invalid state error=%v", err)
	}
	if _, err := service.Get(context.Background(), "tenant", "actor", "not-a-uuid"); !errors.Is(err, ErrInvalidEventEvidence) {
		t.Fatalf("invalid detail identity error=%v", err)
	}
}
