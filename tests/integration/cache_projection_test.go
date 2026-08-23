package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/redis/go-redis/v9"
)

func TestBalanceProjectionIsIdempotentAndNeverRegressesVersion(t *testing.T) {
	harness := RequireHarness(t)
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: harness.RedisAddr, DB: 0})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
	streamName, group, consumer := "test:balance-stream", "test-balance-projectors", "test-projector-1"
	stream, err := events.NewRedisStreams(client, streamName)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := cacheplatform.NewBalanceCache(client, "test:balance-cache", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := cacheplatform.NewAccountAdapter(cache)
	if err != nil {
		t.Fatal(err)
	}
	projector, err := projection.NewBalanceProjector(stream, adapter, projection.Config{Group: group, Consumer: consumer, BatchSize: 10, PendingIdle: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	newest := balanceChangedEvent(t, "event-v2", 2, 7500)
	older := balanceChangedEvent(t, "event-v1", 1, 9000)
	if err := stream.Publish(ctx, newest); err != nil {
		t.Fatal(err)
	}
	if err := stream.Publish(ctx, newest); err != nil {
		t.Fatal(err)
	}
	if err := stream.Publish(ctx, older); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := projector.RunOnce(ctx); err != nil {
			t.Fatalf("project messages: %v", err)
		}
	}
	balance, err := adapter.Get(ctx, testTenantID, testSourceID)
	if err != nil {
		t.Fatalf("read cached balance: %v", err)
	}
	if balance.Version != 2 || balance.AvailableMinor != 7500 {
		t.Fatalf("balance=%#v, want version 2 / minor 7500", balance)
	}
}

func balanceChangedEvent(t *testing.T, eventID string, version, availableMinor int64) outbox.Event {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"event_id": eventID, "event_type": "account.balance.changed.v1", "account_id": testSourceID, "transfer_id": "00000000-0000-0000-0000-000000000555", "currency": "USD", "available_minor": fmt.Sprintf("%d", availableMinor), "balance_version": fmt.Sprintf("%d", version), "occurred_at": time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	return outbox.Event{ID: eventID, TenantID: testTenantID, TransferID: "00000000-0000-0000-0000-000000000555", AccountID: testSourceID, EventType: "account.balance.changed.v1", AggregateVersion: version, Payload: payload, OccurredAt: time.Now().UTC()}
}
