package integration_test

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/redis/go-redis/v9"
)

func TestEnsuringExistingRedisConsumerGroupDoesNotEmitBusyGroupErrors(t *testing.T) {
	address := os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("LEDGERSYNC_TEST_REDIS_ADDR is required for Redis stream integration tests")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	stream := "ledgersync:test:consumer-group"
	group := "projection-test"
	if err := client.Del(ctx, stream).Err(); err != nil {
		t.Fatal(err)
	}
	publisher, err := events.NewRedisStreams(client, stream)
	if err != nil {
		t.Fatal(err)
	}
	before := redisBusyGroupCount(t, ctx, client)
	if err := publisher.EnsureConsumerGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	if err := publisher.EnsureConsumerGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	after := redisBusyGroupCount(t, ctx, client)
	if after != before {
		t.Fatalf("BUSYGROUP errors increased from %d to %d", before, after)
	}
	groups, err := client.XInfoGroups(ctx, stream).Result()
	if err != nil || len(groups) != 1 || groups[0].Name != group {
		t.Fatalf("consumer groups=%#v err=%v", groups, err)
	}
}

func redisBusyGroupCount(t *testing.T, ctx context.Context, client *redis.Client) int64 {
	t.Helper()
	info, err := client.Info(ctx, "errorstats").Result()
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`errorstat_BUSYGROUP:count=([0-9]+)`).FindStringSubmatch(info)
	if len(match) != 2 {
		return 0
	}
	count, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return count
}
