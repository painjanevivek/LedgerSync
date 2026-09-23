// Package bootstrap contains process-neutral dependency wiring shared by
// long-running binaries and request-driven deployment adapters.
package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/projection"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookverification"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/events"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	managedsecrets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/secrets"
	"github.com/redis/go-redis/v9"
)

type batchWorker interface {
	RunOnce(context.Context) (int, error)
}

// WorkerCounts reports bounded work claimed by one drain cycle.
type WorkerCounts struct {
	Outbox               int `json:"outbox"`
	WebhookDeliveries    int `json:"webhook_deliveries"`
	WebhookVerifications int `json:"webhook_verifications"`
	BalanceProjections   int `json:"balance_projections"`
}

func (c WorkerCounts) Total() int {
	return c.Outbox + c.WebhookDeliveries + c.WebhookVerifications + c.BalanceProjections
}

func (c *WorkerCounts) Add(other WorkerCounts) {
	c.Outbox += other.Outbox
	c.WebhookDeliveries += other.WebhookDeliveries
	c.WebhookVerifications += other.WebhookVerifications
	c.BalanceProjections += other.BalanceProjections
}

// WorkerRunner owns the four idempotent batch processors used by both the
// continuous worker and Vercel's authenticated cron drain.
type WorkerRunner struct {
	outbox               batchWorker
	webhookDeliveries    batchWorker
	webhookVerifications batchWorker
	balanceProjections   batchWorker
	streams              *events.RedisStreams
	telemetry            *observability.Telemetry
}

func NewWorkerRunner(ctx context.Context, configuration config.Config, database *sql.DB, redisClient redis.UniversalClient, telemetry *observability.Telemetry) (*WorkerRunner, error) {
	if database == nil || redisClient == nil {
		return nil, errors.New("worker database and Redis clients are required")
	}
	store, err := db.NewOutboxRepository(database, nil, telemetry)
	if err != nil {
		return nil, fmt.Errorf("initialize outbox repository: %w", err)
	}
	streams, err := events.NewRedisStreams(redisClient, "", telemetry)
	if err != nil {
		return nil, fmt.Errorf("initialize Redis streams: %w", err)
	}
	streams.WithMaxLength(configuration.RedisStreamMaxLength)

	hostname, _ := os.Hostname()
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())
	ryewMetrics := observability.NewRYEWMetrics(telemetry)
	outboxWorker, err := outbox.NewWorker(store, streams, ryewMetrics, nil, outbox.Config{WorkerID: workerID})
	if err != nil {
		return nil, fmt.Errorf("initialize outbox worker: %w", err)
	}

	webhookStore, err := db.NewWebhookDeliveryJobRepository(database, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize webhook delivery store: %w", err)
	}
	var webhookKeys webhookdelivery.KeyResolver
	if configuration.Environment == "development" {
		webhookKeys, err = webhookdelivery.NewStaticKeyResolver(configuration.WebhookSigningKeys)
	} else {
		var managedKeys *managedsecrets.AWSSecretsManager
		managedKeys, err = managedsecrets.NewAWSSecretsManager(ctx, configuration.AWSRegion)
		if err == nil {
			webhookKeys, err = webhookdelivery.NewCachedKeyResolver(managedKeys, 5*time.Minute, nil)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("initialize webhook key resolver: %w", err)
	}
	webhookDispatcher, err := webhookdelivery.NewDispatcher(webhookdelivery.NewSecureHTTPClient(), webhookKeys, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize webhook dispatcher: %w", err)
	}
	webhookWorker, err := webhookdelivery.NewWorker(webhookStore, webhookDispatcher, nil, webhookdelivery.Config{WorkerID: workerID + "-webhooks"})
	if err != nil {
		return nil, fmt.Errorf("initialize webhook worker: %w", err)
	}

	verificationStore, err := db.NewWebhookVerificationJobRepository(database, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize webhook verification store: %w", err)
	}
	verificationDispatcher, err := webhookverification.NewDispatcher(webhookdelivery.NewSecureHTTPClient(), webhookKeys, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize webhook verification dispatcher: %w", err)
	}
	verificationWorker, err := webhookverification.NewWorker(verificationStore, verificationDispatcher, nil, webhookverification.Config{WorkerID: workerID + "-webhook-verifications"})
	if err != nil {
		return nil, fmt.Errorf("initialize webhook verification worker: %w", err)
	}

	balanceCache, err := cacheplatform.NewBalanceCache(redisClient, "", 5*time.Minute, telemetry)
	if err != nil {
		return nil, fmt.Errorf("initialize balance cache: %w", err)
	}
	cacheAdapter, err := cacheplatform.NewAccountAdapter(balanceCache)
	if err != nil {
		return nil, fmt.Errorf("initialize balance cache adapter: %w", err)
	}
	projector, err := projection.NewBalanceProjector(streams, cacheAdapter, projection.Config{Group: "balance-cache-v1", Consumer: workerID})
	if err != nil {
		return nil, fmt.Errorf("initialize balance projector: %w", err)
	}
	return &WorkerRunner{outbox: outboxWorker, webhookDeliveries: webhookWorker, webhookVerifications: verificationWorker, balanceProjections: projector, streams: streams, telemetry: telemetry}, nil
}

// RunOnce attempts every independent processor and joins infrastructure
// failures so a problem in one queue does not starve the others.
func (r *WorkerRunner) RunOnce(ctx context.Context) (WorkerCounts, error) {
	var counts WorkerCounts
	var failures []error
	run := func(operation string, worker batchWorker, destination *int) {
		started := time.Now()
		iterationCtx, span := r.telemetry.Start(ctx, "outbox.worker."+operation)
		count, err := worker.RunOnce(iterationCtx)
		span.End()
		r.telemetry.ObserveBoundary(iterationCtx, "worker", operation, started, err)
		*destination = count
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", operation, err))
		}
	}
	run("publish", r.outbox, &counts.Outbox)
	run("webhook_dispatch", r.webhookDeliveries, &counts.WebhookDeliveries)
	run("webhook_verification", r.webhookVerifications, &counts.WebhookVerifications)
	run("project", r.balanceProjections, &counts.BalanceProjections)
	return counts, errors.Join(failures...)
}

func (r *WorkerRunner) ObserveHealth(ctx context.Context) error {
	return r.streams.ObserveHealth(ctx, "balance-cache-v1")
}
