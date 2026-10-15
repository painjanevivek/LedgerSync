package scheduler

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/lease"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"sync/atomic"
	"testing"
	"time"
)

type storeStub struct {
	renewals    atomic.Int32
	claims      atomic.Int32
	completions atomic.Int32
	releases    atomic.Int32
	failFirst   bool
}

func (s *storeStub) Acquire(context.Context, string, string, time.Duration) (int64, error) {
	return 1, nil
}
func (s *storeStub) Renew(context.Context, string, string, int64, time.Duration) error {
	if s.renewals.Add(1) > 1 || s.failFirst {
		return errors.New("database unavailable")
	}
	return nil
}
func (s *storeStub) Claim(context.Context, string, string, int64, time.Duration) (string, error) {
	s.claims.Add(1)
	return "job", nil
}
func (s *storeStub) Complete(context.Context, string, string, int64, string, reconciliation.Result) error {
	s.completions.Add(1)
	return nil
}
func (s *storeStub) Release(context.Context, string, string, int64) error {
	s.releases.Add(1)
	return nil
}

type waitingRunner struct{}

func (waitingRunner) Run(ctx context.Context, _ string) (reconciliation.Result, error) {
	<-ctx.Done()
	return reconciliation.Result{}, ctx.Err()
}
func TestRenewalLossStopsSchedulerWork(t *testing.T) {
	for _, first := range []bool{true, false} {
		store := &storeStub{failFirst: first}
		service, err := New(store, waitingRunner{}, "node", time.Second, 15*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = service.Tick(ctx, "tenant")
		cancel()
		if !errors.Is(err, lease.ErrLost) || store.completions.Load() != 0 || store.releases.Load() != 1 || first && store.claims.Load() != 0 {
			t.Fatalf("unsafe renewal loss: %v claims=%d complete=%d", err, store.claims.Load(), store.completions.Load())
		}
	}
}
