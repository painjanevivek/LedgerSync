package projection

import (
	"context"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
)

type lifecycleStream struct {
	acked string
}

func (*lifecycleStream) EnsureConsumerGroup(context.Context, string) error { return nil }
func (*lifecycleStream) RecoverPending(context.Context, string, string, time.Duration, int64) ([]Message, error) {
	return []Message{{ID: "stream-id", Event: outbox.Event{ID: "event-id", TenantID: "tenant", AccountID: "account", AggregateType: "account", AggregateID: "account", EventType: "account.status.changed.v1", AggregateVersion: 2, Payload: []byte(`{"status":"closed"}`)}}}, nil
}
func (*lifecycleStream) ReadGroup(context.Context, string, string, int64, time.Duration) ([]Message, error) {
	return nil, nil
}
func (s *lifecycleStream) Ack(_ context.Context, _, id string) error { s.acked = id; return nil }

type rejectingBalanceCache struct{ called bool }

func (c *rejectingBalanceCache) Put(context.Context, accounts.Balance) (bool, error) {
	c.called = true
	return false, nil
}

func TestLifecycleOutboxEventAdvancesBalanceConsumerWithoutProjectionMutation(t *testing.T) {
	stream, cache := &lifecycleStream{}, &rejectingBalanceCache{}
	projector, err := NewBalanceProjector(stream, cache, Config{Group: "balance", Consumer: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if count, err := projector.RunOnce(context.Background()); err != nil || count != 1 {
		t.Fatalf("count=%d error=%v", count, err)
	}
	if stream.acked != "stream-id" || cache.called {
		t.Fatalf("acked=%q cache-called=%t", stream.acked, cache.called)
	}
}

func TestReservedBalanceEventPreservesPostedFunds(t *testing.T) {
	event := outbox.Event{ID: "hold-event", TenantID: "tenant", AccountID: "account", EventType: ReservedBalanceChangedEventType, AggregateVersion: 3, Payload: []byte(`{"event_id":"hold-event","event_type":"account.balance.changed.v2","account_id":"account","currency":"INR","available_minor":"700","ledger_minor":"1000","reserved_minor":"300","balance_version":"3","occurred_at":"2026-10-16T00:00:00Z"}`)}
	balance, err := decodeBalance(event)
	if err != nil || balance.LedgerMinor != 1000 || balance.AvailableMinor != 700 || balance.Version != 3 {
		t.Fatalf("reserved projection %+v %v", balance, err)
	}
	event.Payload = []byte(`{"event_id":"hold-event","event_type":"account.balance.changed.v2","account_id":"account","currency":"INR","available_minor":"700","ledger_minor":"1000","reserved_minor":"301","balance_version":"3","occurred_at":"2026-10-16T00:00:00Z"}`)
	if _, err := decodeBalance(event); err == nil {
		t.Fatal("invalid reservation equation projected")
	}
}
