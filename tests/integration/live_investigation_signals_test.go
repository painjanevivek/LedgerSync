package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	platformevents "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/redis/go-redis/v9"
)

func TestLiveInvestigationSignalsAreOrderedBoundedAndEphemeral(t *testing.T) {
	harness := RequireHarness(t)
	client := redis.NewClient(&redis.Options{Addr: harness.RedisAddr})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	namespace := "ledgersync:test:live:" + time.Now().UTC().Format("20060102150405.000000000")
	store, err := platformevents.NewLiveInvestigationSignals(client, namespace)
	if err != nil {
		t.Fatalf("create live signal store: %v", err)
	}
	roomID := "00000000-0000-4000-8000-000000000044"
	signal := investigation.LiveSignal{
		RoomID: roomID, Role: "owner", Sequence: 100, Kind: "offer",
		Payload: json.RawMessage(`{"type":"offer","sdp":"v=0"}`), SentAt: time.Now().UTC(),
	}
	if err := store.Append(ctx, signal); err != nil {
		t.Fatalf("append signal: %v", err)
	}
	if err := store.Append(ctx, signal); !errors.Is(err, investigation.ErrLiveRoomConflict) {
		t.Fatalf("replayed sequence must conflict: %v", err)
	}
	ttl, err := client.TTL(ctx, namespace+":"+roomID+":signals").Result()
	if err != nil || ttl < 34*time.Minute || ttl > investigation.LiveSignalTTL {
		t.Fatalf("unexpected signal TTL=%s error=%v", ttl, err)
	}
	page, err := store.Read(ctx, roomID, "0-0")
	if err != nil || len(page.Signals) != 1 || page.Signals[0].Sequence != 100 || page.Cursor == "0-0" {
		t.Fatalf("read signals=%#v error=%v", page, err)
	}
	if err := store.Presence(ctx, roomID, "owner"); err != nil {
		t.Fatalf("set presence: %v", err)
	}
	present, err := store.IsPresent(ctx, roomID, "owner")
	if err != nil || !present {
		t.Fatalf("read presence=%t error=%v", present, err)
	}
	if err := store.Clear(ctx, roomID); err != nil {
		t.Fatalf("clear ephemeral room state: %v", err)
	}
	page, err = store.Read(ctx, roomID, "0-0")
	if err != nil || len(page.Signals) != 0 {
		t.Fatalf("signals persisted after clear=%#v error=%v", page, err)
	}
}
