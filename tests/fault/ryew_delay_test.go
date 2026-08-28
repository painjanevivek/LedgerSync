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

func TestRequirementBearingReadUsesPrimaryWhenRedisProjectionIsDelayed(t *testing.T) {
	service, database, redisClient := requireFaultDependencies(t, 10000)
	submission, err := service.Submit(context.Background(), faultTransferCommand(t, "fault-ryew-delay-key-0001", "25.00"))
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
	// Simulate an event worker behind the committed transfer: version zero is
	// present in Redis while PostgreSQL already holds the required version.
	if _, err := adapter.Put(context.Background(), accounts.Balance{TenantID: faultTenantID, AccountID: faultSourceID, Currency: "USD", AvailableMinor: 10000, LedgerMinor: 10000, Version: 0, AsOf: time.Now().UTC()}); err != nil {
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
	reader, err := accounts.NewReader(primary, adapter, issuer, accounts.ReaderConfig{MaximumWait: 5 * time.Millisecond, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	result, err := reader.Read(context.Background(), faultTenantID, faultActorID, faultSourceID, requirement)
	if err != nil {
		t.Fatalf("read current balance: %v", err)
	}
	if result.Source != accounts.SourcePrimary || result.Balance.Version < requiredVersion {
		t.Fatalf("result=%#v, want primary version >= %d", result, requiredVersion)
	}
}
