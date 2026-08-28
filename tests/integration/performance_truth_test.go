package integration_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestHealthyPostgreSQLCriticalPathsStayWithinLocalP95Targets(t *testing.T) {
	service, database := requireTransferService(t, 100_000)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	transferDurations := make([]time.Duration, 0, 24)
	for index := 0; index < cap(transferDurations); index++ {
		started := time.Now()
		if _, err := service.Submit(ctx, transferCommand(t, fmt.Sprintf("performance-transfer-%04d", index), "0.01")); err != nil {
			t.Fatalf("submit performance transfer %d: %v", index, err)
		}
		transferDurations = append(transferDurations, time.Since(started))
	}
	transferP95 := nearestRankP95(transferDurations)
	t.Logf("healthy PostgreSQL transfer p95=%s target=<500ms", transferP95)
	if measured := transferP95; measured >= 500*time.Millisecond {
		t.Fatalf("healthy PostgreSQL transfer p95=%s, target <500ms", measured)
	}

	repository, err := db.NewBalanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	current, err := repository.ReadCurrent(ctx, testTenantID, testActorID, testSourceID)
	if err != nil {
		t.Fatal(err)
	}
	cache := &performanceBalanceCache{value: current}
	reader, err := accounts.NewReader(repository, cache, nil, accounts.ReaderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	balanceDurations := make([]time.Duration, 0, 40)
	for index := 0; index < cap(balanceDurations); index++ {
		started := time.Now()
		result, err := reader.Read(ctx, testTenantID, testActorID, testSourceID, "")
		if err != nil {
			t.Fatalf("read performance balance %d: %v", index, err)
		}
		if result.Source != accounts.SourcePrimary || result.Balance.Version != current.Version || result.Balance.AvailableMinor != current.AvailableMinor || result.Balance.LedgerMinor != current.LedgerMinor {
			t.Fatalf("balance performance path returned %#v", result)
		}
		balanceDurations = append(balanceDurations, time.Since(started))
	}
	balanceP95 := nearestRankP95(balanceDurations)
	t.Logf("healthy authoritative PostgreSQL balance p95=%s target=<200ms", balanceP95)
	if measured := balanceP95; measured >= 200*time.Millisecond {
		t.Fatalf("healthy authorized balance p95=%s, target <200ms", measured)
	}
}

type performanceBalanceCache struct{ value accounts.Balance }

func (c *performanceBalanceCache) Get(context.Context, string, string) (accounts.Balance, error) {
	return c.value, nil
}

func (c *performanceBalanceCache) Put(_ context.Context, value accounts.Balance) (bool, error) {
	c.value = value
	return true, nil
}

func nearestRankP95(samples []time.Duration) time.Duration {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	rank := (95*len(ordered) + 99) / 100
	return ordered[rank-1]
}
