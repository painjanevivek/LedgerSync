package webhookdelivery

import (
	"context"
	"errors"
	"testing"
	"time"
)

type workerStore struct {
	jobs      []Job
	completed []Completion
}

func (s *workerStore) Claim(_ context.Context, _ string, _ int, _ time.Duration) ([]Job, error) {
	return s.jobs, nil
}

func (s *workerStore) Complete(_ context.Context, completion Completion) error {
	s.completed = append(s.completed, completion)
	return nil
}

type workerDispatcher struct {
	outcome  Outcome
	err      error
	requests []Delivery
}

func (d *workerDispatcher) Dispatch(_ context.Context, request Delivery) (Outcome, error) {
	d.requests = append(d.requests, request)
	return d.outcome, d.err
}

func TestWorkerRecordsDeliveredAttempt(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	store := &workerStore{jobs: []Job{testJob()}}
	dispatcher := &workerDispatcher{outcome: Outcome{ResponseClass: "http_202"}}
	worker, err := NewWorker(store, dispatcher, func() time.Time { return now }, Config{WorkerID: "worker-1"})
	if err != nil {
		t.Fatal(err)
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 || len(store.completed) != 1 {
		t.Fatalf("processed=%d completions=%d", processed, len(store.completed))
	}
	completion := store.completed[0]
	if completion.Status != StatusDelivered || completion.RetryAt != nil || completion.ResponseClass != "http_202" {
		t.Fatalf("unexpected completion: %#v", completion)
	}
	if got := dispatcher.requests[0]; got.ID != testJob().ID || got.EventID != testJob().EventID || got.EventType != testJob().EventType {
		t.Fatalf("unexpected dispatch request: %#v", got)
	}
}

func TestWorkerSchedulesRetryForRetryableFailure(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	store := &workerStore{jobs: []Job{testJob()}}
	dispatcher := &workerDispatcher{outcome: Outcome{ResponseClass: "network_error", Retryable: true}, err: errors.New("network unavailable")}
	worker, err := NewWorker(store, dispatcher, func() time.Time { return now }, Config{WorkerID: "worker-1", MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	completion := store.completed[0]
	if completion.Status != StatusRetrying || completion.RetryAt == nil || !completion.RetryAt.Equal(now.Add(time.Second)) || completion.ErrorCode != "network_error" {
		t.Fatalf("unexpected retry completion: %#v", completion)
	}
}

func TestWorkerDeadLettersNonRetryableOrExhaustedDelivery(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	job := testJob()
	job.AttemptNumber = 3
	store := &workerStore{jobs: []Job{job}}
	dispatcher := &workerDispatcher{outcome: Outcome{ResponseClass: "http_400"}, err: errors.New("bad request")}
	worker, err := NewWorker(store, dispatcher, func() time.Time { return now }, Config{WorkerID: "worker-1", MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	completion := store.completed[0]
	if completion.Status != StatusDead || completion.RetryAt != nil || completion.ErrorCode != "http_400" {
		t.Fatalf("unexpected dead-letter completion: %#v", completion)
	}
}

func TestWorkerDeadLettersInactiveWebhookWithoutDispatch(t *testing.T) {
	store := &workerStore{jobs: []Job{{ID: "job-1", TenantID: "tenant-1", TransferID: "transfer-1", OutboxEventID: "event-1", WebhookID: "webhook-1", EventID: "event-1", EventType: "transfer.posted", Payload: []byte(`{"event_id":"event-1"}`), AttemptNumber: 1}}}
	dispatcher := &workerDispatcher{}
	worker, err := NewWorker(store, dispatcher, nil, Config{WorkerID: "worker-1"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.requests) != 0 || store.completed[0].Status != StatusDead || store.completed[0].ErrorCode != "endpoint_inactive" {
		t.Fatalf("requests=%d completion=%#v", len(dispatcher.requests), store.completed[0])
	}
}

func testJob() Job {
	return Job{ID: "job-1", TenantID: "tenant-1", TransferID: "transfer-1", OutboxEventID: "event-1", WebhookID: "webhook-1", EventID: "event-1", EventType: "transfer.posted", EndpointURL: "https://partner.example.test/hooks/transfers", SigningKeyReference: "secrets/webhooks/partner", SigningKeyID: "key-2026", Payload: []byte(`{"event_id":"event-1"}`), AttemptNumber: 1}
}
