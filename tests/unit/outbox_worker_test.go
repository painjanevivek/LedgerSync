package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
)

func TestOutboxWorkerReschedulesFailureAndDoesNotMarkItPublished(t *testing.T) {
	store := &fakeOutboxStore{events: []outbox.Event{{ID: "event", TenantID: "tenant", AccountID: "account", EventType: "account.balance.changed.v1", AggregateVersion: 1, Payload: []byte(`{"ok":true}`), AttemptCount: 1}}}
	worker, err := outbox.NewWorker(store, failingPublisher{}, nil, func() time.Time { return time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC) }, outbox.Config{WorkerID: "worker", MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.rescheduled != "event" || store.published != "" || store.dead != "" {
		t.Fatalf("store = %#v", store)
	}
}

func TestOutboxWorkerMarksExhaustedEventDead(t *testing.T) {
	store := &fakeOutboxStore{events: []outbox.Event{{ID: "event", TenantID: "tenant", AccountID: "account", EventType: "account.balance.changed.v1", AggregateVersion: 1, Payload: []byte(`{"ok":true}`), AttemptCount: 2}}}
	worker, _ := outbox.NewWorker(store, failingPublisher{}, nil, time.Now, outbox.Config{WorkerID: "worker", MaxAttempts: 2})
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.dead != "event" || store.published != "" {
		t.Fatalf("store = %#v", store)
	}
}

func TestOutboxWorkerSurfacesRetryPersistenceFailure(t *testing.T) {
	store := &fakeOutboxStore{events: []outbox.Event{{ID: "event", TenantID: "tenant", AccountID: "account", EventType: "account.balance.changed.v1", AggregateVersion: 1, Payload: []byte(`{"ok":true}`), AttemptCount: 1}}, persistErr: errors.New("postgres unavailable")}
	worker, _ := outbox.NewWorker(store, failingPublisher{}, nil, time.Now, outbox.Config{WorkerID: "worker", MaxAttempts: 2})
	if _, err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("retry persistence failure was swallowed")
	}
}

type fakeOutboxStore struct {
	events                       []outbox.Event
	published, rescheduled, dead string
	persistErr                   error
}

func (f *fakeOutboxStore) Claim(context.Context, string, int, time.Duration) ([]outbox.Event, error) {
	return f.events, nil
}
func (f *fakeOutboxStore) MarkPublished(_ context.Context, _ string, eventID string, _ time.Time) error {
	f.published = eventID
	return nil
}
func (f *fakeOutboxStore) Reschedule(_ context.Context, _ string, eventID string, _ time.Time, _ string) error {
	f.rescheduled = eventID
	return f.persistErr
}
func (f *fakeOutboxStore) MarkDead(_ context.Context, _ string, eventID string, _ time.Time, _ string) error {
	f.dead = eventID
	return f.persistErr
}

type failingPublisher struct{}

func (failingPublisher) Publish(context.Context, outbox.Event) error {
	return errors.New("redis unavailable")
}
