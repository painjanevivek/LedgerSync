package bootstrap

import (
	"context"
	"errors"
	"testing"
)

type stubBatchWorker struct {
	count int
	err   error
	calls int
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
