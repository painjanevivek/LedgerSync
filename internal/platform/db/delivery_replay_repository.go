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
var ErrDeliveryReplayIdempotencyConflict = errors.New("delivery replay idempotency key belongs to different input")

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

func (r *DeliveryReplayRepository) Approve(ctx context.Context, approval recovery.DeliveryApproval) (recovery.DeliveryApprovalResult, error) {
	if approval.TenantID == "" || approval.AttemptID == "" || approval.ActorSubjectID == "" || approval.ReasonCode == "" || approval.CorrelationID == "" || approval.IdempotencyKey == "" {
		return recovery.DeliveryApprovalResult{}, errors.New("complete delivery replay approval is required")
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return recovery.DeliveryApprovalResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM delivery_attempts WHERE tenant_id=$1 AND id=$2 AND status='dead')`, approval.TenantID, approval.AttemptID).Scan(&exists); err != nil {
		return recovery.DeliveryApprovalResult{}, err
	}
	if !exists {
		return recovery.DeliveryApprovalResult{}, ErrDeadDeliveryNotFound
	}
	id, err := newUUID()
	if err != nil {
		return recovery.DeliveryApprovalResult{}, err
	}
	now := r.clock().UTC()
	result, err := tx.ExecContext(ctx, `INSERT INTO delivery_replay_actions(id,tenant_id,attempt_id,action,actor_subject_id,reason_code,correlation_id,request_key,created_at)VALUES($1,$2,$3,'approved',$4,$5,$6,$7,$8) ON CONFLICT (tenant_id,attempt_id,action,request_key) DO NOTHING`, id, approval.TenantID, approval.AttemptID, approval.ActorSubjectID, approval.ReasonCode, approval.CorrelationID, approval.IdempotencyKey, now)
	if err != nil {
		return recovery.DeliveryApprovalResult{}, fmt.Errorf("persist delivery replay approval: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return recovery.DeliveryApprovalResult{}, err
	}
	if rows == 0 {
		var actor, reason string
		if err = tx.QueryRowContext(ctx, `SELECT id::text,actor_subject_id,reason_code FROM delivery_replay_actions WHERE tenant_id=$1 AND attempt_id=$2 AND action='approved' AND request_key=$3`, approval.TenantID, approval.AttemptID, approval.IdempotencyKey).Scan(&id, &actor, &reason); err != nil {
			return recovery.DeliveryApprovalResult{}, err
		}
		if actor != approval.ActorSubjectID || reason != approval.ReasonCode {
			return recovery.DeliveryApprovalResult{}, ErrDeliveryReplayIdempotencyConflict
		}
		if err = tx.Commit(); err != nil {
			return recovery.DeliveryApprovalResult{}, err
		}
		return recovery.DeliveryApprovalResult{ApprovalID: id, Replayed: true}, nil
	}
	metadata, _ := json.Marshal(map[string]string{"reason_code": approval.ReasonCode})
	if err = appendControlledAuditPayload(ctx, tx, id, AuditEvent{
		TenantID: approval.TenantID, ActorSubjectID: approval.ActorSubjectID,
		EventType: "delivery.replay_approved", TargetType: "delivery_attempt", TargetID: approval.AttemptID,
		Outcome: "succeeded", CorrelationID: approval.CorrelationID, OccurredAt: now,
	}, metadata); err != nil {
		return recovery.DeliveryApprovalResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return recovery.DeliveryApprovalResult{}, err
	}
	return recovery.DeliveryApprovalResult{ApprovalID: id}, nil
}

func (r *DeliveryReplayRepository) Replay(ctx context.Context, command recovery.DeliveryReplay) (recovery.DeliveryReplayResult, error) {
	if command.TenantID == "" || command.AttemptID == "" || command.ApprovalID == "" || command.ActorSubjectID == "" || command.CorrelationID == "" || command.IdempotencyKey == "" {
		return recovery.DeliveryReplayResult{}, errors.New("complete delivery replay command is required")
	}
	var submission recovery.DeliveryReplayResult
	err := WithSerializableSequence(ctx, r.database, "delivery-replay|"+command.TenantID+"|"+command.AttemptID, 5, func(tx *sql.Tx) error {
		var existingKey sql.NullString
		var existingJob sql.NullString
		existingErr := tx.QueryRowContext(ctx, `SELECT request_key,sanitized_details->>'webhook_delivery_job_id' FROM delivery_replay_actions WHERE tenant_id=$1 AND attempt_id=$2 AND action='executed'`, command.TenantID, command.AttemptID).Scan(&existingKey, &existingJob)
		if existingErr == nil {
			if existingKey.String != command.IdempotencyKey || existingJob.String == "" {
				return ErrDeliveryReplayIdempotencyConflict
			}
			submission = recovery.DeliveryReplayResult{DeliveryJobID: existingJob.String, Replayed: true}
			return nil
		}
		if !errors.Is(existingErr, sql.ErrNoRows) {
			return existingErr
		}

		var approver, reason string
		err := tx.QueryRowContext(ctx, `SELECT actor_subject_id,reason_code FROM delivery_replay_actions WHERE id=$1 AND tenant_id=$2 AND attempt_id=$3 AND action='approved' FOR UPDATE`, command.ApprovalID, command.TenantID, command.AttemptID).Scan(&approver, &reason)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeliveryReplayNotApproved
		}
		if err != nil {
			return err
		}
		if approver == command.ActorSubjectID {
			return ErrReplaySeparationRequired
		}

		var transferID, outboxEventID, kind, endpoint string
		var attemptNumber, latestNumber int
		err = tx.QueryRowContext(ctx, `SELECT transfer_id,COALESCE(outbox_event_id::text,''),delivery_kind,endpoint_reference,attempt_number FROM delivery_attempts WHERE tenant_id=$1 AND id=$2 AND status='dead' FOR UPDATE`, command.TenantID, command.AttemptID).Scan(&transferID, &outboxEventID, &kind, &endpoint, &attemptNumber)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeadDeliveryNotFound
		}
		if err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, `SELECT max(attempt_number) FROM delivery_attempts WHERE tenant_id=$1 AND transfer_id=$2 AND delivery_kind=$3 AND endpoint_reference=$4`, command.TenantID, transferID, kind, endpoint).Scan(&latestNumber); err != nil {
			return err
		}
		if latestNumber != attemptNumber {
			return errors.New("a newer delivery attempt already exists")
		}
		if kind != "webhook" || outboxEventID == "" {
			return errors.New("only event-backed webhook delivery can be replayed")
		}

		newJobID, err := newUUID()
		if err != nil {
			return err
		}
		now := r.clock().UTC()
		result, err := tx.ExecContext(ctx, `INSERT INTO webhook_delivery_jobs(id,tenant_id,transfer_id,outbox_event_id,webhook_id,event_id,event_type,payload,attempt_number,replay_of_attempt_id,available_at,created_at,updated_at)
SELECT $1,$2,$3,event.id,endpoint.id,event.id,'transfer.posted',event.payload,$4,$5,$6,$6,$6
FROM outbox_events event
JOIN developer_webhook_endpoints endpoint ON endpoint.tenant_id=event.tenant_id AND endpoint.id::text=$7
WHERE event.id=$8 AND event.tenant_id=$2 AND event.transfer_id=$3 AND event.event_type='transfer.posted.v1'`, newJobID, command.TenantID, transferID, attemptNumber+1, command.AttemptID, now, endpoint, outboxEventID)
		if err != nil {
			return fmt.Errorf("schedule webhook delivery replay: %w", err)
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			if rowsErr != nil {
				return rowsErr
			}
			return errors.New("webhook replay source no longer has a verified event and endpoint")
		}
		actionID, err := newUUID()
		if err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]string{"approval_id": command.ApprovalID, "approved_by": approver, "webhook_delivery_job_id": newJobID, "reason_code": reason})
		if _, err = tx.ExecContext(ctx, `INSERT INTO delivery_replay_actions(id,tenant_id,attempt_id,action,actor_subject_id,reason_code,correlation_id,request_key,sanitized_details,created_at)VALUES($1,$2,$3,'executed',$4,$5,$6,$7,$8,$9)`, actionID, command.TenantID, command.AttemptID, command.ActorSubjectID, reason, command.CorrelationID, command.IdempotencyKey, details, now); err != nil {
			return fmt.Errorf("persist delivery replay execution: %w", err)
		}
		if err = appendControlledAuditPayload(ctx, tx, actionID, AuditEvent{
			TenantID: command.TenantID, ActorSubjectID: command.ActorSubjectID,
			EventType: "delivery.replayed", TargetType: "delivery_attempt", TargetID: command.AttemptID,
			Outcome: "succeeded", CorrelationID: command.CorrelationID, OccurredAt: now,
		}, details); err != nil {
			return err
		}
		submission = recovery.DeliveryReplayResult{DeliveryJobID: newJobID}
		return nil
	})
	return submission, err
}
