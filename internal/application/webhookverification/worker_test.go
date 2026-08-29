package webhookverification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type workerStore struct {
	jobs        []Job
	completions []Completion
}

func (s *workerStore) Claim(context.Context, string, int, time.Duration) ([]Job, error) {
	return s.jobs, nil
}
func (s *workerStore) Complete(_ context.Context, completion Completion) error {
	s.completions = append(s.completions, completion)
	return nil
}

type workerSender struct {
	outcome Outcome
	err     error
}

func (s workerSender) Verify(context.Context, Job) (Outcome, error) { return s.outcome, s.err }

func TestWorkerActivatesOnlyOnVerifiedProof(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store := &workerStore{jobs: []Job{{ID: "job-1", EndpointURL: "https://partner.example.test/hook", SigningKeyReference: "secret/ref", SigningKeyID: "key-1", Challenge: []byte("challenge-challenge-challenge-challenge"), ExpiresAt: now.Add(time.Minute), AttemptNumber: 1}}}
	worker, err := NewWorker(store, workerSender{outcome: Outcome{ResponseClass: "http_200"}}, func() time.Time { return now }, Config{WorkerID: "worker-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.completions) != 1 || store.completions[0].Status != StatusVerified {
		t.Fatalf("verification completion=%#v", store.completions)
	}
}

func TestWorkerMarksInvalidProofDeadAndNetworkFailureRetryable(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	base := Job{ID: "job-1", EndpointURL: "https://partner.example.test/hook", SigningKeyReference: "secret/ref", SigningKeyID: "key-1", Challenge: []byte("challenge-challenge-challenge-challenge"), ExpiresAt: now.Add(time.Minute), AttemptNumber: 1}
	for name, sender := range map[string]workerSender{
		"invalid proof":   {outcome: Outcome{ResponseClass: "invalid_proof"}, err: errors.New("invalid proof")},
		"network failure": {outcome: Outcome{ResponseClass: "network_error", Retryable: true}, err: errors.New("network")},
	} {
		t.Run(name, func(t *testing.T) {
			store := &workerStore{jobs: []Job{base}}
			worker, err := NewWorker(store, sender, func() time.Time { return now }, Config{WorkerID: "worker-1"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := worker.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := store.completions[0]
			if name == "invalid proof" && got.Status != StatusDead {
				t.Fatalf("invalid proof completion=%#v", got)
			}
			if name == "network failure" && (got.Status != StatusRetrying || got.RetryAt == nil) {
				t.Fatalf("network retry completion=%#v", got)
			}
		})
	}
}
