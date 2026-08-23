package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

// OutboxRepository is the PostgreSQL delivery-state adapter. It never changes
// financial records; it only coordinates already-committed outbox rows.
type OutboxRepository struct {
	database  *sql.DB
	clock     func() time.Time
	telemetry *observability.Telemetry
}

func NewOutboxRepository(database *sql.DB, clock func() time.Time, telemetry ...*observability.Telemetry) (*OutboxRepository, error) {
	if database == nil {
		return nil, errors.New("outbox database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &OutboxRepository{database: database, clock: clock, telemetry: firstTelemetry(telemetry)}, nil
}

func (r *OutboxRepository) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]outbox.Event, error) {
	if workerID == "" || limit <= 0 || lease <= 0 {
		return nil, errors.New("valid outbox claim arguments are required")
	}
	now := r.clock().UTC()
	rows, err := r.database.QueryContext(ctx, `
WITH candidates AS (
    SELECT id
    FROM outbox_events
    WHERE published_at IS NULL
      AND dead_at IS NULL
      AND available_at <= $1
      AND (claimed_until IS NULL OR claimed_until <= $1)
    ORDER BY available_at, created_at, id
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events AS event
SET claim_owner = $3,
    claimed_until = $4,
    attempt_count = event.attempt_count + 1
FROM candidates
WHERE event.id = candidates.id
RETURNING event.id, event.tenant_id, event.transfer_id, event.account_id, event.event_type,
          event.aggregate_version, event.payload, event.occurred_at, event.attempt_count`, now, limit, workerID, now.Add(lease))
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var events []outbox.Event
	for rows.Next() {
		var event outbox.Event
		if err := rows.Scan(&event.ID, &event.TenantID, &event.TransferID, &event.AccountID, &event.EventType, &event.AggregateVersion, &event.Payload, &event.OccurredAt, &event.AttemptCount); err != nil {
			return nil, fmt.Errorf("scan claimed outbox event: %w", err)
		}
		if r.telemetry != nil && now.After(event.OccurredAt) {
			r.telemetry.ObserveOutboxAge(ctx, now.Sub(event.OccurredAt))
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed outbox events: %w", err)
	}
	return events, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, workerID, eventID string, at time.Time) error {
	return r.updateClaimed(ctx, `
UPDATE outbox_events
SET published_at = $3, claim_owner = NULL, claimed_until = NULL
WHERE id = $1 AND claim_owner = $2 AND published_at IS NULL AND dead_at IS NULL`, workerID, eventID, at, "mark outbox event published")
}

func (r *OutboxRepository) Reschedule(ctx context.Context, workerID, eventID string, availableAt time.Time, errorCode string) error {
	return r.updateClaimed(ctx, `
UPDATE outbox_events
SET available_at = $3, last_error_code = $4, claim_owner = NULL, claimed_until = NULL
WHERE id = $1 AND claim_owner = $2 AND published_at IS NULL AND dead_at IS NULL`, workerID, eventID, availableAt, errorCode, "reschedule outbox event")
}

func (r *OutboxRepository) MarkDead(ctx context.Context, workerID, eventID string, at time.Time, errorCode string) error {
	return r.updateClaimed(ctx, `
UPDATE outbox_events
SET dead_at = $3, last_error_code = $4, claim_owner = NULL, claimed_until = NULL
WHERE id = $1 AND claim_owner = $2 AND published_at IS NULL AND dead_at IS NULL`, workerID, eventID, at, errorCode, "mark outbox event dead")
}

func (r *OutboxRepository) updateClaimed(ctx context.Context, statement, workerID, eventID string, value any, args ...string) error {
	queryArgs := []any{eventID, workerID, value}
	if len(args) > 1 {
		queryArgs = append(queryArgs, args[0])
	}
	result, err := r.database.ExecContext(ctx, statement, queryArgs...)
	if err != nil {
		return err
	}
	operation := args[len(args)-1]
	return requireOneRow(result, operation)
}
