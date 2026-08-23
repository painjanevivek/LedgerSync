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

var ErrDeadDeliveryNotFound = errors.New("dead delivery attempt not found")
var ErrDeliveryReplayNotApproved = errors.New("delivery replay is not approved")

type DeliveryReplayRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewDeliveryReplayRepository(database *sql.DB, clock func() time.Time) (*DeliveryReplayRepository, error) {
	if database == nil {
		return nil, errors.New("delivery replay database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &DeliveryReplayRepository{database: database, clock: clock}, nil
}

func (r *DeliveryReplayRepository) Inspect(ctx context.Context, tenantID, attemptID string) (recovery.DeadDelivery, error) {
	var item recovery.DeadDelivery
	err := r.database.QueryRowContext(ctx, `SELECT id,tenant_id,transfer_id,COALESCE(outbox_event_id::text,''),delivery_kind,endpoint_reference,attempt_number,COALESCE(sanitized_error_code,''),completed_at FROM delivery_attempts WHERE tenant_id=$1 AND id=$2 AND status='dead'`, tenantID, attemptID).Scan(&item.AttemptID, &item.TenantID, &item.TransferID, &item.OutboxEventID, &item.Kind, &item.EndpointReference, &item.AttemptNumber, &item.ErrorCode, &item.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrDeadDeliveryNotFound
	}
	return item, err
}

func (r *DeliveryReplayRepository) Approve(ctx context.Context, approval recovery.DeliveryApproval) error {
	if approval.TenantID == "" || approval.AttemptID == "" || approval.ActorSubjectID == "" || approval.ReasonCode == "" || approval.CorrelationID == "" {
		return errors.New("complete delivery replay approval is required")
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_attempts WHERE tenant_id=$1 AND id=$2 AND status='dead')`, approval.TenantID, approval.AttemptID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrDeadDeliveryNotFound
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	now := r.clock().UTC()
	result, err := tx.ExecContext(ctx, `INSERT INTO delivery_replay_actions(id,tenant_id,attempt_id,action,actor_subject_id,reason_code,correlation_id,created_at)VALUES($1,$2,$3,'approved',$4,$5,$6,$7) ON CONFLICT (attempt_id,action,correlation_id) DO NOTHING`, id, approval.TenantID, approval.AttemptID, approval.ActorSubjectID, approval.ReasonCode, approval.CorrelationID, now)
	if err != nil {
		return fmt.Errorf("persist delivery replay approval: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var actor, reason string
		if err = tx.QueryRowContext(ctx, `SELECT actor_subject_id,reason_code FROM delivery_replay_actions WHERE attempt_id=$1 AND action='approved' AND correlation_id=$2`, approval.AttemptID, approval.CorrelationID).Scan(&actor, &reason); err != nil {
			return err
		}
		if actor != approval.ActorSubjectID || reason != approval.ReasonCode {
			return errors.New("delivery replay approval correlation is already bound to different input")
		}
		return tx.Commit()
	}
	metadata, _ := json.Marshal(map[string]string{"reason_code": approval.ReasonCode})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)VALUES($1,$2,$3,'delivery.replay_approved','delivery_attempt',$4,'succeeded',$5,$6,$7)`, id, approval.TenantID, approval.ActorSubjectID, approval.AttemptID, approval.CorrelationID, metadata, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *DeliveryReplayRepository) Replay(ctx context.Context, command recovery.DeliveryReplay) (string, error) {
	if command.TenantID == "" || command.AttemptID == "" || command.ActorSubjectID == "" || command.CorrelationID == "" {
		return "", errors.New("complete delivery replay command is required")
	}
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var approver, reason string
	err = tx.QueryRowContext(ctx, `SELECT actor_subject_id,reason_code FROM delivery_replay_actions WHERE tenant_id=$1 AND attempt_id=$2 AND action='approved' AND correlation_id=$3 ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, command.TenantID, command.AttemptID, command.CorrelationID).Scan(&approver, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDeliveryReplayNotApproved
	}
	if err != nil {
		return "", err
	}
	if approver == command.ActorSubjectID {
		return "", ErrReplaySeparationRequired
	}

	var transferID, outboxEventID, kind, endpoint string
	var attemptNumber, latestNumber int
	err = tx.QueryRowContext(ctx, `SELECT transfer_id,COALESCE(outbox_event_id::text,''),delivery_kind,endpoint_reference,attempt_number FROM delivery_attempts WHERE tenant_id=$1 AND id=$2 AND status='dead' FOR UPDATE`, command.TenantID, command.AttemptID).Scan(&transferID, &outboxEventID, &kind, &endpoint, &attemptNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDeadDeliveryNotFound
	}
	if err != nil {
		return "", err
	}
	if err = tx.QueryRowContext(ctx, `SELECT max(attempt_number) FROM delivery_attempts WHERE tenant_id=$1 AND transfer_id=$2 AND delivery_kind=$3 AND endpoint_reference=$4`, command.TenantID, transferID, kind, endpoint).Scan(&latestNumber); err != nil {
		return "", err
	}
	if latestNumber != attemptNumber {
		return "", errors.New("a newer delivery attempt already exists")
	}

	newAttemptID, err := newUUID()
	if err != nil {
		return "", err
	}
	now := r.clock().UTC()
	if _, err = tx.ExecContext(ctx, `INSERT INTO delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,due_at)VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5,$6,$7,'pending',$8)`, newAttemptID, command.TenantID, transferID, outboxEventID, kind, endpoint, attemptNumber+1, now); err != nil {
		return "", fmt.Errorf("schedule delivery replay: %w", err)
	}
	actionID, err := newUUID()
	if err != nil {
		return "", err
	}
	details, _ := json.Marshal(map[string]string{"approved_by": approver, "new_attempt_id": newAttemptID, "reason_code": reason})
	if _, err = tx.ExecContext(ctx, `INSERT INTO delivery_replay_actions(id,tenant_id,attempt_id,action,actor_subject_id,reason_code,correlation_id,sanitized_details,created_at)VALUES($1,$2,$3,'executed',$4,$5,$6,$7,$8)`, actionID, command.TenantID, command.AttemptID, command.ActorSubjectID, reason, command.CorrelationID, details, now); err != nil {
		return "", fmt.Errorf("persist delivery replay execution: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)VALUES($1,$2,$3,'delivery.replayed','delivery_attempt',$4,'succeeded',$5,$6,$7)`, actionID, command.TenantID, command.ActorSubjectID, command.AttemptID, command.CorrelationID, details, now); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return newAttemptID, nil
}
