package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

func (r *OutboxReplayRepository) Inspect(ctx context.Context, tenantID, eventID string) (recovery.DeadOutbox, error) {
	var item recovery.DeadOutbox
	err := r.database.QueryRowContext(ctx, `SELECT id,tenant_id,transfer_id,account_id,event_type,attempt_count,COALESCE(last_error_code,''),occurred_at,dead_at FROM outbox_events WHERE tenant_id=$1 AND id=$2 AND dead_at IS NOT NULL AND published_at IS NULL`, tenantID, eventID).Scan(&item.EventID, &item.TenantID, &item.TransferID, &item.AccountID, &item.EventType, &item.AttemptCount, &item.LastErrorCode, &item.OccurredAt, &item.DeadAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrDeadOutboxNotFound
	}
	return item, err
}

func (r *OutboxReplayRepository) Approve(ctx context.Context, approval recovery.Approval) error {
	if approval.TenantID == "" || approval.EventID == "" || approval.ActorSubjectID == "" || approval.ReasonCode == "" || approval.CorrelationID == "" {
		return errors.New("complete replay approval is required")
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_events WHERE tenant_id=$1 AND id=$2 AND dead_at IS NOT NULL AND published_at IS NULL)`, approval.TenantID, approval.EventID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrDeadOutboxNotFound
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	now := r.clock().UTC()
	result, err := tx.ExecContext(ctx, `INSERT INTO outbox_replay_actions (id,tenant_id,event_id,action,actor_subject_id,reason_code,correlation_id,created_at) VALUES ($1,$2,$3,'approved',$4,$5,$6,$7) ON CONFLICT (event_id,action,correlation_id) DO NOTHING`, id, approval.TenantID, approval.EventID, approval.ActorSubjectID, approval.ReasonCode, approval.CorrelationID, now)
	if err != nil {
		return fmt.Errorf("persist replay approval: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var actor, reason string
		if err = tx.QueryRowContext(ctx, `SELECT actor_subject_id,reason_code FROM outbox_replay_actions WHERE event_id=$1 AND action='approved' AND correlation_id=$2`, approval.EventID, approval.CorrelationID).Scan(&actor, &reason); err != nil {
			return err
		}
		if actor != approval.ActorSubjectID || reason != approval.ReasonCode {
			return errors.New("replay approval correlation is already bound to different input")
		}
		return tx.Commit()
	}
	metadata, _ := json.Marshal(map[string]string{"reason_code": approval.ReasonCode})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,$3,'outbox.replay_approved','outbox_event',$4,'succeeded',$5,$6,$7)`, id, approval.TenantID, approval.ActorSubjectID, approval.EventID, approval.CorrelationID, metadata, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OutboxReplayRepository) Replay(ctx context.Context, command recovery.Replay) error {
	if command.TenantID == "" || command.EventID == "" || command.ActorSubjectID == "" || command.CorrelationID == "" {
		return errors.New("complete replay command is required")
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var approver, reason string
	err = tx.QueryRowContext(ctx, `SELECT actor_subject_id,reason_code FROM outbox_replay_actions WHERE tenant_id=$1 AND event_id=$2 AND action='approved' AND correlation_id=$3 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, command.TenantID, command.EventID, command.CorrelationID).Scan(&approver, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrReplayNotApproved
	}
	if err != nil {
		return err
	}
	if approver == command.ActorSubjectID {
		return ErrReplaySeparationRequired
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox_events SET dead_at=NULL,last_error_code='approved_replay',attempt_count=0,available_at=$3,claim_owner=NULL,claimed_until=NULL WHERE tenant_id=$1 AND id=$2 AND dead_at IS NOT NULL AND published_at IS NULL`, command.TenantID, command.EventID, r.clock().UTC())
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ErrDeadOutboxNotFound
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	now := r.clock().UTC()
	if _, err = tx.ExecContext(ctx, `INSERT INTO outbox_replay_actions (id,tenant_id,event_id,action,actor_subject_id,reason_code,correlation_id,created_at) VALUES ($1,$2,$3,'executed',$4,$5,$6,$7)`, id, command.TenantID, command.EventID, command.ActorSubjectID, reason, command.CorrelationID, now); err != nil {
		return fmt.Errorf("persist replay execution: %w", err)
	}
	metadata, _ := json.Marshal(map[string]string{"reason_code": reason, "approved_by": approver})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,$3,'outbox.replayed','outbox_event',$4,'succeeded',$5,$6,$7)`, id, command.TenantID, command.ActorSubjectID, command.EventID, command.CorrelationID, metadata, now); err != nil {
		return err
	}
	return tx.Commit()
}
