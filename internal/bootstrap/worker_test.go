package bootstrap

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"reflect"
	"testing"
)

type stubBatchWorker struct {
	count int
	err   error
	calls int
}

func TestWorkerRunnerKeepsProgressCallbacksAroundEveryQueue(t *testing.T) {
	var events []string
	failure := errors.New("synthetic publisher failure")
	runner := &WorkerRunner{outbox: &stubBatchWorker{err: failure}, webhookDeliveries: &stubBatchWorker{}, webhookVerifications: &stubBatchWorker{}, balanceProjections: &stubBatchWorker{}, options: WorkerRunnerOptions{
		OnStarted: func(queue string) { events = append(events, "start:"+queue) },
		OnCompleted: func(queue string, err error) {
			result := "ok"
			if err != nil {
				result = "failed"
			}
			events = append(events, "complete:"+queue+":"+result)
		},
	}}
	if _, err := runner.RunOnce(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("lost worker error: %v", err)
	}
	expected := []string{}
	for i, queue := range []string{observability.WorkerQueueOutbox, observability.WorkerQueueWebhookDelivery, observability.WorkerQueueWebhookVerification, observability.WorkerQueueProjection} {
		result := "ok"
		if i == 0 {
			result = "failed"
		}
		expected = append(expected, "start:"+queue, "complete:"+queue+":"+result)
	}
	if !reflect.DeepEqual(events, expected) {
		t.Fatalf("progress callbacks=%v want %v", events, expected)
	}
}

func (s *stubBatchWorker) RunOnce(context.Context) (int, error) {
	s.calls++
	return s.count, s.err
}

func TestWorkerRunnerAttemptsEveryIndependentProcessor(t *testing.T) {
	publisher := &stubBatchWorker{count: 2, err: errors.New("publish unavailable")}
	deliveries := &stubBatchWorker{count: 3}
	verifications := &stubBatchWorker{count: 4}
	projections := &stubBatchWorker{count: 5}
	runner := &WorkerRunner{
		outbox:               publisher,
		webhookDeliveries:    deliveries,
		webhookVerifications: verifications,
		balanceProjections:   projections,
	}
	counts, err := runner.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected the publisher failure to be returned")
	}
	if counts.Total() != 14 {
		t.Fatalf("expected all claimed work to be counted, got %#v", counts)
	}
	if publisher.calls != 1 || deliveries.calls != 1 || verifications.calls != 1 || projections.calls != 1 {
		t.Fatalf("expected every processor to run once: %d %d %d %d", publisher.calls, deliveries.calls, verifications.calls, projections.calls)
	}
}
