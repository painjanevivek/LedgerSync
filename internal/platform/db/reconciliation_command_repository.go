package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
)

type storedReconciliationFailure struct {
	ErrorCode   string `json:"error_code"`
	ActiveRunID string `json:"active_run_id,omitempty"`
}

func (r *ReconciliationRepository) RunCommand(ctx context.Context, command reconciliation.RunCommand, fingerprint [sha256.Size]byte) (submission reconciliation.CommandSubmission, err error) {
	started := time.Now()
	ctx, span := r.start(ctx, "db.reconciliation.command")
	defer func() { span.End(); r.observe(ctx, started, err) }()

	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return submission, fmt.Errorf("%w: begin command: %w", reconciliation.ErrCommandUnavailable, err)
	}
	defer func() { _ = tx.Rollback() }()

	stored, replayed, denial, replayedActiveRunID, err := reserveReconciliationCommand(ctx, tx, command, fingerprint)
	if err != nil {
		if errors.Is(err, idempotency.ErrInProgress) {
			activeRunID, activeCorrelation, requestedAt, owns, loadErr := loadActiveReconciliationCommand(ctx, tx, command)
			if loadErr != nil {
				return submission, classifyReconciliationCommandError(loadErr)
			}
			if commitErr := tx.Commit(); commitErr != nil {
				return submission, fmt.Errorf("%w: commit active lookup: %w", reconciliation.ErrCommandUnavailable, commitErr)
			}
			if !owns {
				return submission, reconciliation.ErrCommandInProgress
			}
			command.CorrelationID, command.OccurredAt = activeCorrelation, requestedAt
			return r.executeReconciliationCommand(ctx, command, activeRunID)
		}
		return submission, classifyReconciliationCommandError(err)
	}
	if replayed {
		if err := tx.Commit(); err != nil {
			return submission, fmt.Errorf("%w: commit replay: %w", reconciliation.ErrCommandUnavailable, err)
		}
		return reconciliation.CommandSubmission{Result: stored, Replayed: true, Denial: denial, ActiveRunID: replayedActiveRunID}, nil
	}

	runID, err := newUUID()
	if err != nil {
		return submission, err
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO reconciliation_run_commands(tenant_id,run_id,actor_subject_id,idempotency_key,correlation_id,lease_expires_at,requested_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id) DO NOTHING`, command.TenantID, runID, command.ActorSubjectID, command.IdempotencyKey, command.CorrelationID, command.OccurredAt.Add(2*time.Minute), command.OccurredAt)
	if err != nil {
		return submission, classifyReconciliationCommandError(err)
	}
	created, err := inserted.RowsAffected()
	if err != nil {
		return submission, classifyReconciliationCommandError(err)
	}
	if created != 1 {
		var activeRunID, activeActor, activeKey, activeCorrelation string
		var leaseExpiresAt time.Time
		if err := tx.QueryRowContext(ctx, `SELECT run_id::text,actor_subject_id,idempotency_key,correlation_id::text,lease_expires_at FROM reconciliation_run_commands WHERE tenant_id=$1`, command.TenantID).Scan(&activeRunID, &activeActor, &activeKey, &activeCorrelation, &leaseExpiresAt); err != nil {
			return submission, classifyReconciliationCommandError(err)
		}
		if !leaseExpiresAt.After(time.Now().UTC()) {
			lockFree, lockErr := acquireReconciliationLock(ctx, tx, command.TenantID)
			if lockErr != nil {
				return submission, classifyReconciliationCommandError(lockErr)
			}
			if lockFree {
				stale := reconciliation.RunCommand{TenantID: command.TenantID, ActorSubjectID: activeActor, IdempotencyKey: activeKey, CorrelationID: activeCorrelation, OccurredAt: command.OccurredAt}
				if err := persistReconciliationDenial(ctx, tx, stale, "response_unknown", activeRunID); err != nil {
					return submission, classifyReconciliationCommandError(err)
				}
				if err := storeReconciliationFailure(ctx, tx, stale, "response_unknown", activeRunID, 504); err != nil {
					return submission, classifyReconciliationCommandError(err)
				}
				if _, err := tx.ExecContext(ctx, `DELETE FROM reconciliation_run_commands WHERE tenant_id=$1 AND run_id=$2`, command.TenantID, activeRunID); err != nil {
					return submission, classifyReconciliationCommandError(err)
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO reconciliation_run_commands(tenant_id,run_id,actor_subject_id,idempotency_key,correlation_id,lease_expires_at,requested_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, command.TenantID, runID, command.ActorSubjectID, command.IdempotencyKey, command.CorrelationID, command.OccurredAt.Add(2*time.Minute), command.OccurredAt); err != nil {
					return submission, classifyReconciliationCommandError(err)
				}
				if err := persistReconciliationRequest(ctx, tx, command, runID); err != nil {
					return submission, classifyReconciliationCommandError(err)
				}
				if err := tx.Commit(); err != nil {
					return submission, fmt.Errorf("%w: commit recovered request: %w", reconciliation.ErrResponseUnknown, err)
				}
				return r.executeReconciliationCommand(ctx, command, runID)
			}
		}
		if err := persistReconciliationDenial(ctx, tx, command, "already_running", activeRunID); err != nil {
			return submission, classifyReconciliationCommandError(err)
		}
		if err := storeReconciliationFailure(ctx, tx, command, "already_running", activeRunID, 409); err != nil {
			return submission, classifyReconciliationCommandError(err)
		}
		if err := tx.Commit(); err != nil {
			return submission, fmt.Errorf("%w: commit denial: %w", reconciliation.ErrResponseUnknown, err)
		}
		return reconciliation.CommandSubmission{Denial: "already_running", ActiveRunID: activeRunID}, nil
	}

	if err := persistReconciliationRequest(ctx, tx, command, runID); err != nil {
		return submission, classifyReconciliationCommandError(err)
	}
	if err := tx.Commit(); err != nil {
		return submission, fmt.Errorf("%w: commit request: %w", reconciliation.ErrResponseUnknown, err)
	}
	return r.executeReconciliationCommand(ctx, command, runID)
}

func (r *ReconciliationRepository) executeReconciliationCommand(ctx context.Context, command reconciliation.RunCommand, runID string) (reconciliation.CommandSubmission, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return reconciliation.CommandSubmission{}, classifyReconciliationCommandError(err)
	}
	defer func() { _ = tx.Rollback() }()
	// This is deliberately the first statement: a run never takes its financial
	// snapshot before proving non-queued tenant execution authority.
	locked, err := acquireReconciliationLock(ctx, tx, command.TenantID)
	if err != nil {
		return reconciliation.CommandSubmission{}, classifyReconciliationCommandError(err)
	}
	if !locked {
		return reconciliation.CommandSubmission{}, reconciliation.ErrCommandInProgress
	}
	var owns bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reconciliation_run_commands WHERE tenant_id=$1 AND run_id=$2 AND actor_subject_id=$3 AND idempotency_key=$4)`, command.TenantID, runID, command.ActorSubjectID, command.IdempotencyKey).Scan(&owns); err != nil {
		return reconciliation.CommandSubmission{}, classifyReconciliationCommandError(err)
	}
	if !owns {
		return reconciliation.CommandSubmission{}, reconciliation.ErrCommandInProgress
	}
	result, err := r.reconcileTx(ctx, tx, command.TenantID, command.ActorSubjectID, command.CorrelationID, runID, command.OccurredAt)
	if err != nil {
		return reconciliation.CommandSubmission{}, classifyReconciliationCommandError(err)
	}
	if err := storeReconciliationOutcome(ctx, tx, command, result); err != nil {
		return reconciliation.CommandSubmission{}, classifyReconciliationCommandError(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM reconciliation_run_commands WHERE tenant_id=$1 AND run_id=$2`, command.TenantID, runID); err != nil {
		return reconciliation.CommandSubmission{}, classifyReconciliationCommandError(err)
	}
	if err := tx.Commit(); err != nil {
		return reconciliation.CommandSubmission{}, fmt.Errorf("%w: commit outcome: %w", reconciliation.ErrResponseUnknown, err)
	}
	return reconciliation.CommandSubmission{Result: result}, nil
}

func loadActiveReconciliationCommand(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand) (string, string, time.Time, bool, error) {
	var runID, actorID, key, correlationID string
	var requestedAt time.Time
	err := tx.QueryRowContext(ctx, `SELECT run_id::text,actor_subject_id,idempotency_key,correlation_id::text,requested_at FROM reconciliation_run_commands WHERE tenant_id=$1`, command.TenantID).Scan(&runID, &actorID, &key, &correlationID, &requestedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", time.Time{}, false, nil
	}
	if err != nil {
		return "", "", time.Time{}, false, err
	}
	return runID, correlationID, requestedAt, actorID == command.ActorSubjectID && key == command.IdempotencyKey, nil
}

func reserveReconciliationCommand(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand, fingerprint [sha256.Size]byte) (reconciliation.Result, bool, string, string, error) {
	var storedFingerprint, body []byte
	var state string
	err := tx.QueryRowContext(ctx, `
INSERT INTO idempotency_requests (tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,expires_at)
VALUES ($1,$2,$3,$4,$5,'in_progress',$6)
ON CONFLICT (tenant_id,actor_subject_id,operation,idempotency_key) DO NOTHING
RETURNING request_fingerprint,state,response_body`, command.TenantID, command.ActorSubjectID, reconciliation.RunOperation, command.IdempotencyKey, fingerprint[:], command.OccurredAt.AddDate(0, 0, 30)).Scan(&storedFingerprint, &state, &body)
	if err == nil {
		return reconciliation.Result{}, false, "", "", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return reconciliation.Result{}, false, "", "", fmt.Errorf("reserve reconciliation idempotency: %w", err)
	}
	err = tx.QueryRowContext(ctx, `SELECT request_fingerprint,state,response_body FROM idempotency_requests WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 FOR UPDATE`, command.TenantID, command.ActorSubjectID, reconciliation.RunOperation, command.IdempotencyKey).Scan(&storedFingerprint, &state, &body)
	if err != nil {
		return reconciliation.Result{}, false, "", "", fmt.Errorf("load reconciliation idempotency: %w", err)
	}
	if len(storedFingerprint) != sha256.Size {
		return reconciliation.Result{}, false, "", "", errors.New("stored reconciliation idempotency fingerprint is malformed")
	}
	var existing [sha256.Size]byte
	copy(existing[:], storedFingerprint)
	resolution, err := idempotency.Resolve(&idempotency.Existing{Fingerprint: existing, State: idempotency.State(state)}, fingerprint)
	if err != nil {
		return reconciliation.Result{}, false, "", "", err
	}
	if resolution != idempotency.ResolutionReplay || len(body) == 0 {
		return reconciliation.Result{}, false, "", "", idempotency.ErrInProgress
	}
	if state == string(idempotency.StateFailed) {
		var failure storedReconciliationFailure
		if err := json.Unmarshal(body, &failure); err != nil || failure.ErrorCode != "already_running" && failure.ErrorCode != "response_unknown" {
			return reconciliation.Result{}, false, "", "", errors.New("stored reconciliation denial is malformed")
		}
		return reconciliation.Result{}, true, failure.ErrorCode, failure.ActiveRunID, nil
	}
	var result reconciliation.Result
	if err := json.Unmarshal(body, &result); err != nil {
		return reconciliation.Result{}, false, "", "", fmt.Errorf("decode reconciliation outcome: %w", err)
	}
	return result, true, "", "", nil
}

func storeReconciliationOutcome(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand, result reconciliation.Result) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return updateReconciliationIdempotency(ctx, tx, command, "completed", 201, body)
}

func storeReconciliationFailure(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand, code, activeRunID string, status int) error {
	body, err := json.Marshal(storedReconciliationFailure{ErrorCode: code, ActiveRunID: activeRunID})
	if err != nil {
		return err
	}
	return updateReconciliationIdempotency(ctx, tx, command, "failed", status, body)
}

func updateReconciliationIdempotency(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand, state string, status int, body []byte) error {
	result, err := tx.ExecContext(ctx, `UPDATE idempotency_requests SET state=$5,response_status=$6,response_body=$7::jsonb,completed_at=$8 WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 AND state='in_progress'`, command.TenantID, command.ActorSubjectID, reconciliation.RunOperation, command.IdempotencyKey, state, status, body, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("store reconciliation idempotency: %w", err)
	}
	return requireOneRow(result, "store reconciliation idempotency")
}

func persistReconciliationRequest(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand, runID string) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	metadata, _ := json.Marshal(map[string]string{"operation": reconciliation.RunOperation, "scope": reconciliationScope})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,$3,'reconciliation.requested','reconciliation_run',$4,'allowed',$5,$6,$7)`, id, command.TenantID, command.ActorSubjectID, runID, command.CorrelationID, metadata, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("persist reconciliation request audit: %w", err)
	}
	return nil
}

func persistReconciliationDenial(ctx context.Context, tx *sql.Tx, command reconciliation.RunCommand, code, activeRunID string) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	metadata, _ := json.Marshal(map[string]string{"operation": reconciliation.RunOperation, "denial_code": code, "scope": reconciliationScope})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,$3,'reconciliation.command_denied','reconciliation_run',$4,'denied',$5,$6,$7)`, id, command.TenantID, command.ActorSubjectID, activeRunID, command.CorrelationID, metadata, command.OccurredAt)
	if err != nil {
		return fmt.Errorf("persist reconciliation denial audit: %w", err)
	}
	return nil
}

func classifyReconciliationCommandError(err error) error {
	if errors.Is(err, idempotency.ErrConflict) || errors.Is(err, idempotency.ErrInProgress) {
		return err
	}
	return fmt.Errorf("%w: %w", reconciliation.ErrCommandUnavailable, err)
}
