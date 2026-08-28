package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
)

func reserveAccountCommand(ctx context.Context, tx *sql.Tx, envelope commandEnvelope, fingerprint [sha256.Size]byte) (accounts.CommandResult, bool, error, error) {
	result := accounts.CommandResult{}
	var storedFingerprint []byte
	var state string
	var body []byte
	err := tx.QueryRowContext(ctx, `
INSERT INTO idempotency_requests (tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,expires_at)
VALUES ($1,$2,$3,$4,$5,'in_progress',$6)
ON CONFLICT (tenant_id,actor_subject_id,operation,idempotency_key) DO NOTHING
RETURNING request_fingerprint,state,response_body`, envelope.TenantID, envelope.ActorID, envelope.Operation, envelope.Key, fingerprint[:], envelope.OccurredAt.AddDate(0, 0, 30)).Scan(&storedFingerprint, &state, &body)
	if err == nil {
		return result, false, nil, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, false, nil, fmt.Errorf("reserve account command idempotency: %w", err)
	}
	err = tx.QueryRowContext(ctx, `
SELECT request_fingerprint,state,response_body
FROM idempotency_requests
WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4
FOR UPDATE`, envelope.TenantID, envelope.ActorID, envelope.Operation, envelope.Key).Scan(&storedFingerprint, &state, &body)
	if err != nil {
		return result, false, nil, fmt.Errorf("load account command idempotency: %w", err)
	}
	if len(storedFingerprint) != sha256.Size {
		return result, false, nil, errors.New("stored account idempotency fingerprint is malformed")
	}
	var stored [sha256.Size]byte
	copy(stored[:], storedFingerprint)
	resolution, err := idempotency.Resolve(&idempotency.Existing{Fingerprint: stored, State: idempotency.State(state)}, fingerprint)
	if err != nil {
		return result, false, nil, err
	}
	if resolution != idempotency.ResolutionReplay || len(body) == 0 {
		return result, false, nil, idempotency.ErrInProgress
	}
	if state == string(idempotency.StateFailed) {
		var failure storedAccountFailure
		if err := json.Unmarshal(body, &failure); err != nil {
			return result, false, nil, fmt.Errorf("decode failed account idempotency outcome: %w", err)
		}
		denial := accountDenialError(failure.ErrorCode)
		if denial == nil {
			return result, false, nil, errors.New("stored account denial code is malformed")
		}
		return result, true, denial, nil
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, false, nil, fmt.Errorf("decode account idempotency outcome: %w", err)
	}
	return result, true, nil, nil
}

func storeAccountOutcome(ctx context.Context, tx *sql.Tx, envelope commandEnvelope, result accounts.CommandResult, status int) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal account idempotency outcome: %w", err)
	}
	updated, err := tx.ExecContext(ctx, `
UPDATE idempotency_requests
SET state='completed',response_status=$5,response_body=$6::jsonb,completed_at=$7
WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 AND state='in_progress'`, envelope.TenantID, envelope.ActorID, envelope.Operation, envelope.Key, status, body, envelope.OccurredAt)
	if err != nil {
		return fmt.Errorf("store account idempotency outcome: %w", err)
	}
	return requireOneRow(updated, "store account idempotency outcome")
}

type storedAccountFailure struct {
	ErrorCode string `json:"error_code"`
}

func storeAccountFailure(ctx context.Context, tx *sql.Tx, envelope commandEnvelope, code string, status int) error {
	body, err := json.Marshal(storedAccountFailure{ErrorCode: code})
	if err != nil {
		return fmt.Errorf("marshal account idempotency failure: %w", err)
	}
	updated, err := tx.ExecContext(ctx, `
UPDATE idempotency_requests
SET state='failed',response_status=$5,response_body=$6::jsonb,completed_at=$7
WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 AND state='in_progress'`, envelope.TenantID, envelope.ActorID, envelope.Operation, envelope.Key, status, body, envelope.OccurredAt)
	if err != nil {
		return fmt.Errorf("store account idempotency failure: %w", err)
	}
	return requireOneRow(updated, "store account idempotency failure")
}
