package db

import (
	"context"
	"database/sql"
	"errors"
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

func (r *DeliveryReplayRepository) Inspect(ctx context.Context, tenantID, actorID, attemptID string) (recovery.DeadDelivery, error) {
	var item recovery.DeadDelivery
	tx, err := beginTenantRead(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: actorID})
	if err != nil {
		return item, err
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `SELECT id,tenant_id,transfer_id,COALESCE(outbox_event_id::text,''),delivery_kind,endpoint_reference,attempt_number,COALESCE(sanitized_error_code,''),completed_at FROM delivery_attempts WHERE tenant_id=$1 AND id=$2 AND status='dead'`, tenantID, attemptID).Scan(&item.AttemptID, &item.TenantID, &item.TransferID, &item.OutboxEventID, &item.Kind, &item.EndpointReference, &item.AttemptNumber, &item.ErrorCode, &item.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrDeadDeliveryNotFound
	}
	return item, err
}

func (r *DeliveryReplayRepository) Approve(ctx context.Context, approval recovery.DeliveryApproval) (result recovery.DeliveryApprovalResult, err error) {
	if approval.TenantID == "" || approval.ActorSubjectID == "" || approval.AttemptID == "" || approval.CorrelationID == "" || approval.IdempotencyKey == "" {
		return result, errors.New("complete delivery replay approval is required")
	}
	err = WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: approval.TenantID, ActorID: approval.ActorSubjectID}, "delivery-replay|"+approval.TenantID+"|"+approval.AttemptID, 5, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT result_id,replayed FROM public.controlled_delivery_recovery_v1($1,$2,$3,'approve',NULL,$4,$5,$6,$7)`, approval.TenantID, approval.ActorSubjectID, approval.AttemptID, approval.IdempotencyKey, approval.ReasonCode, approval.CorrelationID, r.clock().UTC()).Scan(&result.ApprovalID, &result.Replayed)
		return classifyRecoveryCommandError(err, true)
	})
	return result, err
}
func (r *DeliveryReplayRepository) Replay(ctx context.Context, command recovery.DeliveryReplay) (result recovery.DeliveryReplayResult, err error) {
	if command.TenantID == "" || command.ActorSubjectID == "" || command.AttemptID == "" || command.ApprovalID == "" || command.CorrelationID == "" || command.IdempotencyKey == "" {
		return result, errors.New("complete delivery replay command is required")
	}
	err = WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: command.TenantID, ActorID: command.ActorSubjectID}, "delivery-replay|"+command.TenantID+"|"+command.AttemptID, 5, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT result_id,replayed FROM public.controlled_delivery_recovery_v1($1,$2,$3,'execute',$4,$5,'',$6,$7)`, command.TenantID, command.ActorSubjectID, command.AttemptID, command.ApprovalID, command.IdempotencyKey, command.CorrelationID, r.clock().UTC()).Scan(&result.DeliveryJobID, &result.Replayed)
		return classifyRecoveryCommandError(err, true)
	})
	return result, err
}
