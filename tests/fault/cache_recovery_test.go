package fault_test

import (
	"context"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestRedisLossUsesPrimaryAndRebuildsTheBalanceCache(t *testing.T) {
	service, database, redisClient := requireFaultDependencies(t, 10000)
	submission, err := service.Submit(context.Background(), faultTransferCommand(t, "fault-cache-recovery-key-0001", "25.00"))
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	requiredVersion := submission.Result.MinimumBalanceVersions[faultSourceID]
	cache, err := cacheplatform.NewBalanceCache(redisClient, "fault:balances", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := cacheplatform.NewAccountAdapter(cache)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := consistency.NewIssuer(consistency.Key{ID: "test-current", Secret: []byte("01234567890123456789012345678901")}, nil, nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	requirement, err := issuer.Issue(faultTenantID, faultSourceID, requiredVersion)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := db.NewBalanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := accounts.NewReader(primary, adapter, issuer, accounts.ReaderConfig{MaximumWait: time.Millisecond, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if err := redisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	result, err := reader.Read(context.Background(), faultTenantID, faultActorID, faultSourceID, requirement)
	if err != nil {
		t.Fatalf("read after redis loss: %v", err)
	}
	if result.Source != accounts.SourcePrimary || result.Balance.Version < requiredVersion {
		t.Fatalf("result=%#v, want primary version >= %d", result, requiredVersion)
	}
	rebuilt, err := adapter.Get(context.Background(), faultTenantID, faultSourceID)
	if err != nil {
		t.Fatalf("read rebuilt cache: %v", err)
	}
	if rebuilt.Version != result.Balance.Version {
		t.Fatalf("rebuilt cache version=%d, want %d", rebuilt.Version, result.Balance.Version)
	}
}

func TestForgedHighVersionRedisBalanceCannotOverridePostgreSQLTruth(t *testing.T) {
	_, database, redisClient := requireFaultDependencies(t, 10000)
	cache, err := cacheplatform.NewBalanceCache(redisClient, "fault:forged-balances", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := cacheplatform.NewAccountAdapter(cache)
	if err != nil {
		t.Fatal(err)
	}
	forged := accounts.Balance{
		TenantID: faultTenantID, AccountID: faultSourceID, Currency: "EUR",
		AvailableMinor: 999_999_999, LedgerMinor: 999_999_999, Version: 999_999,
		AsOf: time.Now().UTC(),
	}
	if written, err := adapter.Put(context.Background(), forged); err != nil || !written {
		t.Fatalf("seed forged redis balance: written=%t err=%v", written, err)
	}
	primary, err := db.NewBalanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	authoritative, err := primary.ReadCurrent(context.Background(), faultTenantID, faultActorID, faultSourceID)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := accounts.NewReader(primary, adapter, nil, accounts.ReaderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.Read(context.Background(), faultTenantID, faultActorID, faultSourceID, "")
	if err != nil {
		t.Fatalf("read with forged redis balance: %v", err)
	}
	if result.Source != accounts.SourcePrimary || result.Balance != authoritative {
		t.Fatalf("result=%#v, want PostgreSQL %#v; forged Redis=%#v", result, authoritative, forged)
	}
}
