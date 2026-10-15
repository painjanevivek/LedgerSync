package integration_test

import (
	"context"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/redis/go-redis/v9"
	"os"
	"testing"
	"time"
)

func TestCronWorkerRequiresSeparateRestrictedLogin(t *testing.T) {
	service, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(1)
	if err := db.RequireWorkloadRole(ctx, api.db, "ledgersync_api"); err != nil {
		t.Fatal(err)
	}
	if err := db.RequireWorkloadRole(ctx, admin, "ledgersync_worker"); !errors.Is(err, db.ErrWorkloadRoleBinding) {
		t.Fatal("owner credentials accepted", err)
	}
	if err := db.RequireWorkloadRole(ctx, api.db, "ledgersync_worker"); !errors.Is(err, db.ErrWorkloadRoleBinding) {
		t.Fatal("API credentials accepted as worker", err)
	}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")})
	defer func() { _ = client.Close() }()
	configuration := config.Config{Environment: "development", NodeID: "optional-cron-test", CronSecret: "synthetic-cron-secret-with-at-least-32-bytes", CronWorkerDatabaseURL: "configured-separate-worker-login"} //gitleaks:allow -- fixed synthetic test credential
	if _, err := bootstrap.NewCronWorkerRunner(ctx, configuration, api.db, client, nil); !errors.Is(err, db.ErrWorkloadRoleBinding) {
		t.Fatal("optional runner accepted the API pool", err)
	}
	runner, err := bootstrap.NewCronWorkerRunner(ctx, configuration, worker.db, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Submit(ctx, transferCommand(t, "optional-cron-financial-command", "1.00")); err != nil {
		t.Fatal(err)
	}
	counts, err := runner.RunOnce(ctx)
	if err != nil || counts.Outbox != 3 {
		t.Fatal("restricted optional worker failed", err)
	}
	if countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 1 {
		t.Fatal("optional worker changed financial effect count")
	}
}
