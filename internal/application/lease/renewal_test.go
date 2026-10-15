package lease

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRenewalCancelsWorkOnLoss(t *testing.T) {
	var calls atomic.Int64
	err := Run(context.Background(), 30*time.Millisecond, func(context.Context) error {
		if calls.Add(1) > 1 {
			return errors.New("replaced")
		}
		return nil
	}, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	if !errors.Is(err, ErrLost) {
		t.Fatalf("lost lease: %v", err)
	}
}
func TestRenewalChecksBeforeWork(t *testing.T) {
	called := false
	err := Run(context.Background(), time.Second, func(context.Context) error { return errors.New("expired") }, func(context.Context) error { called = true; return nil })
	if !errors.Is(err, ErrLost) || called {
		t.Fatal("work started after lease loss")
	}
}
