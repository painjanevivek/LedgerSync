package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/redis/go-redis/v9"
)

// NewCronWorkerRunner is the optional request-driven worker boundary. The API
// pool cannot satisfy it. The caller owns this separate pool's lifetime.
func NewCronWorkerRunner(ctx context.Context, configuration config.Config, workerDatabase *sql.DB, client redis.UniversalClient, telemetry *observability.Telemetry) (*WorkerRunner, error) {
	if configuration.CronSecret == "" || configuration.CronWorkerDatabaseURL == "" {
		return nil, errors.New("explicit cron credentials and separate worker binding required")
	}
	if err := db.RequireWorkloadRole(ctx, workerDatabase, "ledgersync_worker"); err != nil {
		return nil, err
	}
	return NewWorkerRunnerWithOptions(ctx, configuration, workerDatabase, client, telemetry, WorkerRunnerOptions{NodeRole: "worker"})
}
