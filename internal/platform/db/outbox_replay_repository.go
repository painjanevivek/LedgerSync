package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
)

var ErrDeadOutboxNotFound = errors.New("dead outbox event not found")
var ErrReplayNotApproved = errors.New("outbox replay is not approved")
var ErrReplaySeparationRequired = errors.New("replay operator must differ from approver")

type OutboxReplayRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewOutboxReplayRepository(database *sql.DB, clock func() time.Time) (*OutboxReplayRepository, error) {
	if database == nil {
		return nil, errors.New("outbox replay database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &OutboxReplayRepository{database: database, clock: clock}, nil
}

func (r *OutboxReplayRepository) Inspect(ctx context.Context, tenantID, actorID, eventID string) (recovery.DeadOutbox, error) {
	var item recovery.DeadOutbox
	tx, err := beginTenantRead(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: actorID})
	if err != nil {
		return item, err
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `SELECT id,tenant_id,COALESCE(transfer_id::text,''),COALESCE(account_id::text,''),event_type,attempt_count,COALESCE(last_error_code,''),occurred_at,dead_at FROM outbox_events WHERE tenant_id=$1 AND id=$2 AND dead_at IS NOT NULL AND published_at IS NULL`, tenantID, eventID).Scan(&item.EventID, &item.TenantID, &item.TransferID, &item.AccountID, &item.EventType, &item.AttemptCount, &item.LastErrorCode, &item.OccurredAt, &item.DeadAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrDeadOutboxNotFound
	}
	return item, err
}

func (r *OutboxReplayRepository) Approve(ctx context.Context, approval recovery.Approval) error {
	return r.command(ctx, approval.TenantID, approval.ActorSubjectID, approval.EventID, "approve", approval.ReasonCode, approval.CorrelationID)
}
func (r *OutboxReplayRepository) Replay(ctx context.Context, command recovery.Replay) error {
	return r.command(ctx, command.TenantID, command.ActorSubjectID, command.EventID, "execute", "", command.CorrelationID)
}
func (r *OutboxReplayRepository) command(ctx context.Context, tenantID, actorID, eventID, action, reason, correlation string) error {
	if tenantID == "" || actorID == "" || eventID == "" || correlation == "" {
		return errors.New("complete replay command is required")
	}
	return WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: actorID}, "outbox-replay|"+tenantID+"|"+eventID, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT public.controlled_outbox_recovery_v1($1,$2,$3,$4,$5,$6,$7)`, tenantID, actorID, eventID, action, reason, correlation, r.clock().UTC())
		return classifyRecoveryCommandError(err, false)
	})
}
