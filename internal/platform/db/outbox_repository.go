package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/outbox"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"time"
)

var ErrStaleClaim = errors.New("claim owner, generation, or expiry is no longer valid")

// OutboxRepository coordinates committed events with database time and a new
// generation for each claim. It never changes financial records.
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
func (r *OutboxRepository) Claim(ctx context.Context, workerID string, limit int, duration time.Duration) ([]outbox.Event, error) {
	if workerID == "" || limit < 1 || limit > 100 || duration <= 0 || duration > 10*time.Minute {
		return nil, errors.New("bounded outbox claim is required")
	}
	rows, err := r.database.QueryContext(ctx, `SELECT * FROM public.controlled_claim_outbox_v1($1,$2,$3)`, limit, workerID, duration.Seconds())
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []outbox.Event
	for rows.Next() {
		var event outbox.Event
		var node, parent sql.NullString
		var timestamp sql.NullInt64
		if err := rows.Scan(&event.ID, &event.TenantID, &event.TransferID, &event.AccountID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.AggregateVersion, &event.Payload, &event.OccurredAt, &event.AttemptCount, &event.ClaimGeneration, &node, &timestamp, &parent); err != nil {
			return nil, err
		}
		if node.Valid && timestamp.Valid {
			event.Causal = &outbox.CausalMetadata{NodeID: node.String, Timestamp: timestamp.Int64, ParentReference: parent.String}
		}
		if r.telemetry != nil && r.clock().After(event.OccurredAt) {
			r.telemetry.ObserveOutboxAge(ctx, r.clock().Sub(event.OccurredAt))
		}
		result = append(result, event)
	}
	return result, rows.Err()
}
func (r *OutboxRepository) RenewClaim(ctx context.Context, owner, id string, generation int64, duration time.Duration) error {
	return renewWorkerClaim(ctx, r.database, "outbox", owner, id, generation, duration)
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, owner, id string, at time.Time, generation int64) error {
	return r.complete(ctx, owner, id, at, generation, "published", "")
}
func (r *OutboxRepository) Reschedule(ctx context.Context, owner, id string, at time.Time, code string, generation int64) error {
	return r.complete(ctx, owner, id, at, generation, "retrying", code)
}
func (r *OutboxRepository) MarkDead(ctx context.Context, owner, id string, at time.Time, code string, generation int64) error {
	return r.complete(ctx, owner, id, at, generation, "dead", code)
}
func (r *OutboxRepository) complete(ctx context.Context, owner, id string, at time.Time, generation int64, status, code string) error {
	tx, err := beginClaimTransaction(ctx, r.database, "outbox", id, owner, generation, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var completed bool
	if err := tx.QueryRowContext(ctx, `SELECT public.controlled_complete_outbox_v1($1,$2,$3,$4,$5,$6)`, id, owner, generation, at, status, code).Scan(&completed); err != nil {
		return err
	}
	if !completed {
		return ErrStaleClaim
	}
	return tx.Commit()
}
func claimUpdate(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrStaleClaim
	}
	return nil
}
