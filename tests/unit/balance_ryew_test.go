package unit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
)

func TestBalanceReadWaitsForRequiredVersionThenUsesCache(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	issuer, err := consistency.NewIssuer(consistency.Key{ID: "current", Secret: []byte("01234567890123456789012345678901")}, nil, func() time.Time { return now }, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	requirement, err := issuer.Issue("tenant", "account", 2)
	if err != nil {
		t.Fatal(err)
	}
	cache := &memoryBalanceCache{balance: balance(1)}
	reader, err := accounts.NewReader(failingBalanceRepository{}, cache, issuer, accounts.ReaderConfig{MaximumWait: 100 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(15 * time.Millisecond); cache.set(balance(2)) }()
	result, err := reader.Read(context.Background(), "tenant", "actor", "account", requirement)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if result.Source != accounts.SourceCache || result.Balance.Version != 2 {
		t.Fatalf("result = %#v, want cache version 2", result)
	}
}

func TestBalanceReadFallsBackToPrimaryAndNeverReturnsStaleCache(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	issuer, _ := consistency.NewIssuer(consistency.Key{ID: "current", Secret: []byte("01234567890123456789012345678901")}, nil, func() time.Time { return now }, time.Minute)
	requirement, _ := issuer.Issue("tenant", "account", 3)
	reader, err := accounts.NewReader(staticBalanceRepository{value: balance(3)}, &memoryBalanceCache{balance: balance(2)}, issuer, accounts.ReaderConfig{MaximumWait: time.Millisecond, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.Read(context.Background(), "tenant", "actor", "account", requirement)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if result.Source != accounts.SourcePrimary || result.Balance.Version != 3 {
		t.Fatalf("result = %#v, want primary version 3", result)
	}
}

func TestBalanceReadReturnsTruthfulAvailabilityErrorWhenPrimaryFails(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	issuer, _ := consistency.NewIssuer(consistency.Key{ID: "current", Secret: []byte("01234567890123456789012345678901")}, nil, func() time.Time { return now }, time.Minute)
	requirement, _ := issuer.Issue("tenant", "account", 2)
	reader, err := accounts.NewReader(failingBalanceRepository{}, &memoryBalanceCache{balance: balance(1)}, issuer, accounts.ReaderConfig{MaximumWait: time.Millisecond, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = reader.Read(context.Background(), "tenant", "actor", "account", requirement)
	if !errors.Is(err, accounts.ErrCurrentBalanceUnavailable) {
		t.Fatalf("error = %v, want current balance unavailable", err)
	}
}

func TestConsistencyRequirementCannotBeUsedForAnotherAccount(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	issuer, _ := consistency.NewIssuer(consistency.Key{ID: "current", Secret: []byte("01234567890123456789012345678901")}, nil, func() time.Time { return now }, time.Minute)
	requirement, _ := issuer.Issue("tenant", "account-a", 1)
	reader, _ := accounts.NewReader(staticBalanceRepository{value: balance(1)}, &memoryBalanceCache{balance: balance(1)}, issuer, accounts.ReaderConfig{})
	_, err := reader.Read(context.Background(), "tenant", "actor", "account-b", requirement)
	if err == nil {
		t.Fatal("expected requirement/account mismatch")
	}
}

type staticBalanceRepository struct{ value accounts.Balance }

func (s staticBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	return s.value, nil
}

type failingBalanceRepository struct{}

func (f failingBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	return accounts.Balance{}, errors.New("primary unavailable")
}

type memoryBalanceCache struct {
	mu      sync.RWMutex
	balance accounts.Balance
}

func (m *memoryBalanceCache) Get(context.Context, string, string) (accounts.Balance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.balance, nil
}
func (m *memoryBalanceCache) set(value accounts.Balance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance = value
}
func balance(version int64) accounts.Balance {
	return accounts.Balance{TenantID: "tenant", AccountID: "account", Currency: "USD", AvailableMinor: 100, LedgerMinor: 100, Version: version, AsOf: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}
}
