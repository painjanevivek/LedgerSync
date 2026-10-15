package integration_test

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/lease"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"sync"
	"testing"
	"time"
)

func TestExpiredOutboxOwnerCannotAcknowledge(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	if _, err := service.Submit(context.Background(), transferCommand(t, "expired-worker-claim-test-01", "1.00")); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE outbox_events SET available_at=clock_timestamp()`); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewOutboxRepository(database, time.Now)
	events, err := repository.Claim(context.Background(), "expired-worker", 1, 50*time.Millisecond)
	if err != nil || len(events) != 1 {
		t.Fatalf("claim: %v count=%d", err, len(events))
	}
	time.Sleep(80 * time.Millisecond)
	if err := repository.MarkPublished(context.Background(), "expired-worker", events[0].ID, time.Now(), events[0].ClaimGeneration); err == nil {
		t.Fatal("expired claim was acknowledged")
	}
}

func TestOutboxGenerationRejectsPreviousClaimFromSameOwner(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	if _, err := service.Submit(context.Background(), transferCommand(t, "replaced-worker-claim-test", "1.00")); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewOutboxRepository(database, func() time.Time { return time.Now().Add(24 * time.Hour) })
	if _, err := database.Exec(`UPDATE outbox_events SET available_at=clock_timestamp()`); err != nil {
		t.Fatal(err)
	}
	first, err := repository.Claim(context.Background(), "same-owner", 1, time.Second)
	if err != nil || len(first) != 1 {
		t.Fatalf("first claim %v", err)
	}
	if _, err := database.Exec(`UPDATE outbox_events SET available_at=clock_timestamp()+interval '1 hour' WHERE id<>$1;`, first[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE outbox_events SET claimed_until=clock_timestamp()-interval '1 second' WHERE id=$1`, first[0].ID); err != nil {
		t.Fatal(err)
	}
	second, err := repository.Claim(context.Background(), "same-owner", 1, time.Second)
	if err != nil || len(second) != 1 {
		t.Fatalf("second claim %v", err)
	}
	if second[0].ClaimGeneration <= first[0].ClaimGeneration {
		t.Fatal("generation did not increase")
	}
	old := first[0]
	for _, err := range []error{
		repository.MarkPublished(context.Background(), "same-owner", old.ID, time.Now(), old.ClaimGeneration),
		repository.MarkDead(context.Background(), "same-owner", old.ID, time.Now(), "publish_failed", old.ClaimGeneration),
		repository.Reschedule(context.Background(), "same-owner", old.ID, time.Now(), "publish_failed", old.ClaimGeneration),
		repository.RenewClaim(context.Background(), "same-owner", old.ID, old.ClaimGeneration, time.Second),
	} {
		if !errors.Is(err, db.ErrStaleClaim) {
			t.Fatalf("stale claim changed state: %v", err)
		}
	}
	if err := repository.MarkPublished(context.Background(), "same-owner", second[0].ID, time.Now(), second[0].ClaimGeneration); err != nil {
		t.Fatal(err)
	}
}

func TestOutboxRenewalKeepsLongTaskAndParallelClaimsSeparate(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	if _, err := service.Submit(context.Background(), transferCommand(t, "parallel-worker-claim-test", "1.00")); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE outbox_events SET available_at=clock_timestamp()`); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewOutboxRepository(database, time.Now)
	ids := sync.Map{}
	var wg sync.WaitGroup
	for _, owner := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			events, err := repository.Claim(context.Background(), owner, 1, 500*time.Millisecond)
			if err != nil || len(events) != 1 {
				t.Errorf("parallel claim %v count=%d", err, len(events))
				return
			}
			event := events[0]
			if _, exists := ids.LoadOrStore(event.ID, owner); exists {
				t.Error("simultaneous claims overlapped")
			}
			err = lease.Run(context.Background(), 500*time.Millisecond, func(c context.Context) error {
				return repository.RenewClaim(c, owner, event.ID, event.ClaimGeneration, 500*time.Millisecond)
			}, func(c context.Context) error {
				select {
				case <-c.Done():
					return c.Err()
				case <-time.After(1100 * time.Millisecond):
					return nil
				}
			})
			if err != nil {
				t.Error(err)
				return
			}
			if err := repository.MarkPublished(context.Background(), owner, event.ID, time.Now(), event.ClaimGeneration); err != nil {
				t.Error(err)
			}
		}(owner)
	}
	wg.Wait()
}
