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

func TestBalanceReadNeverReturnsForgedHighVersionCache(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	issuer, err := consistency.NewIssuer(consistency.Key{ID: "current", Secret: []byte("01234567890123456789012345678901")}, nil, func() time.Time { return now }, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	requirement, err := issuer.Issue("tenant", "account", 2)
	if err != nil {
		t.Fatal(err)
	}
	forged := balance(999)
	forged.AvailableMinor, forged.LedgerMinor, forged.Currency = 9_999_999, 9_999_999, "EUR"
	cache := &memoryBalanceCache{balance: forged}
	repository := &countingBalanceRepository{value: balance(2)}
	reader, err := accounts.NewReader(repository, cache, issuer, accounts.ReaderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.Read(context.Background(), "tenant", "actor", "account", requirement)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if result.Source != accounts.SourcePrimary || result.Balance != balance(2) {
		t.Fatalf("result = %#v, want authoritative primary balance", result)
	}
	if repository.projectionReads != 1 || cache.gets != 0 {
		t.Fatalf("projection_reads=%d cache_gets=%d, want one authoritative query and no cache truth read", repository.projectionReads, cache.gets)
	}
}

func TestBalanceReadUsesPrimaryAndNeverReturnsStaleCache(t *testing.T) {
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
	forged := balance(999)
	forged.AvailableMinor = 9_999_999
	reader, err := accounts.NewReader(failingBalanceRepository{}, &memoryBalanceCache{balance: forged}, issuer, accounts.ReaderConfig{})
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

func TestBalanceCacheNeverBypassesAccountAuthorization(t *testing.T) {
	reader, err := accounts.NewReader(unauthorizedBalanceRepository{}, &memoryBalanceCache{balance: balance(9)}, nil, accounts.ReaderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(context.Background(), "tenant", "another-actor", "account", ""); !errors.Is(err, errBalanceNotAuthorized) || !errors.Is(err, accounts.ErrCurrentBalanceUnavailable) {
		t.Fatalf("warm cache authorization error = %v", err)
	}
	if _, err := reader.Read(context.Background(), "tenant", "another-actor", "account", "malformed-requirement"); !errors.Is(err, errBalanceNotAuthorized) || !errors.Is(err, accounts.ErrCurrentBalanceUnavailable) {
		t.Fatalf("authorization must remain non-disclosing before requirement validation: %v", err)
	}
}

func TestWarmBalanceCacheStillPerformsOneAuthoritativeProjectionRead(t *testing.T) {
	repository := &countingBalanceRepository{value: balance(9)}
	cache := &memoryBalanceCache{balance: balance(9)}
	reader, err := accounts.NewReader(repository, cache, nil, accounts.ReaderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.Read(context.Background(), "tenant", "actor", "account", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != accounts.SourcePrimary || repository.projectionReads != 1 || cache.gets != 0 {
		t.Fatalf("result=%#v projection_reads=%d cache_gets=%d", result, repository.projectionReads, cache.gets)
	}
}

type countingBalanceRepository struct {
	value           accounts.Balance
	projectionReads int
}

func (r *countingBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	r.projectionReads++
	return r.value, nil
}

type staticBalanceRepository struct{ value accounts.Balance }

func (s staticBalanceRepository) Authorize(context.Context, string, string, string) error { return nil }

func (s staticBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	return s.value, nil
}

type failingBalanceRepository struct{}

func (f failingBalanceRepository) Authorize(context.Context, string, string, string) error {
	return nil
}

func (f failingBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	return accounts.Balance{}, errors.New("primary unavailable")
}

type unauthorizedBalanceRepository struct{}

var errBalanceNotAuthorized = errors.New("balance account not found or not authorized")

func (unauthorizedBalanceRepository) Authorize(context.Context, string, string, string) error {
	return errBalanceNotAuthorized
}
func (unauthorizedBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	return accounts.Balance{}, errBalanceNotAuthorized
}

type memoryBalanceCache struct {
	mu      sync.RWMutex
	balance accounts.Balance
	gets    int
}

func (m *memoryBalanceCache) Get(context.Context, string, string) (accounts.Balance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gets++
	return m.balance, nil
}
func (m *memoryBalanceCache) set(value accounts.Balance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance = value
}
func (m *memoryBalanceCache) Put(_ context.Context, value accounts.Balance) (bool, error) {
	m.set(value)
	return true, nil
}
func balance(version int64) accounts.Balance {
	return accounts.Balance{TenantID: "tenant", AccountID: "account", Currency: "USD", AvailableMinor: 100, LedgerMinor: 100, Version: version, AsOf: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}
}
