package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// Harness connects integration tests to isolated, externally provisioned
// Postgres and Redis instances. CI supplies disposable containers; local tests
// skip rather than silently use a developer's financial database.
type Harness struct {
	DatabaseURL string
	RedisAddr   string
}

func RequireHarness(t *testing.T) Harness {
	t.Helper()
	databaseURL := os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")
	redisAddr := os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")
	if databaseURL == "" || redisAddr == "" {
		t.Skip("integration dependencies are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: databaseURL})
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return Harness{DatabaseURL: databaseURL, RedisAddr: redisAddr}
}
