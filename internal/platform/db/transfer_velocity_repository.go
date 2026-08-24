package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
)

type transferPolicy struct {
	currency                             string
	minimum, maximum                     int64
	actorLimit, sourceLimit, tenantLimit int64
}

type velocityTotals struct {
	actor, source, tenant int64
}

func (r *TransferRepository) validateTransferPolicy(ctx context.Context, tx *sql.Tx, command transfers.Command) error {
	if command.Amount.Currency().Code != r.pilotCurrency {
		return ErrUnsupportedPilotCurrency
	}
	policy, err := loadTransferPolicy(ctx, tx, command.TenantID)
	if err != nil {
		return err
	}
	if policy.currency != r.pilotCurrency || policy.currency != command.Amount.Currency().Code {
		return ErrUnsupportedPilotCurrency
	}
	minor := command.Amount.Minor()
	if minor < policy.minimum {
		return ErrTransferBelowMinimum
	}
	if minor > policy.maximum {
		return ErrTransferAboveMaximum
	}
	if err := ensureVelocityTotals(ctx, tx, command); err != nil {
		return err
	}
	if err := pruneExpiredVelocity(ctx, tx, command.TenantID, command.OccurredAt.UTC()); err != nil {
		return err
	}
	totals, err := loadVelocityTotals(ctx, tx, command)
	if err != nil {
		return err
	}
	if totals.actor > policy.actorLimit-minor {
		return ErrActorVelocityExceeded
	}
	if totals.source > policy.sourceLimit-minor {
		return ErrSourceVelocityExceeded
	}
	if totals.tenant > policy.tenantLimit-minor {
		return ErrTenantVelocityExceeded
	}
	return nil
}

func loadTransferPolicy(ctx context.Context, tx *sql.Tx, tenantID string) (transferPolicy, error) {
	var policy transferPolicy
	err := tx.QueryRowContext(ctx, `
SELECT currency, minimum_transfer_minor, maximum_transfer_minor,
       actor_rolling_24h_minor, source_account_rolling_24h_minor,
       tenant_rolling_24h_minor
FROM tenant_transfer_policies
WHERE tenant_id = $1
FOR UPDATE`, tenantID).Scan(
		&policy.currency, &policy.minimum, &policy.maximum,
		&policy.actorLimit, &policy.sourceLimit, &policy.tenantLimit,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return transferPolicy{}, ErrTenantPolicyMissing
	}
	if err != nil {
		return transferPolicy{}, fmt.Errorf("load tenant transfer policy: %w", err)
	}
	return policy, nil
}

func ensureVelocityTotals(ctx context.Context, tx *sql.Tx, command transfers.Command) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO transfer_velocity_totals (
  tenant_id, dimension_type, dimension_reference, total_minor, updated_at
) VALUES
  ($1::uuid, 'tenant', $1::uuid::text, 0, $4),
  ($1::uuid, 'actor', $2, 0, $4),
  ($1::uuid, 'source', $3, 0, $4)
ON CONFLICT (tenant_id, dimension_type, dimension_reference) DO NOTHING`,
		command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.OccurredAt.UTC())
	return wrap("ensure transfer velocity totals", err)
}

func pruneExpiredVelocity(ctx context.Context, tx *sql.Tx, tenantID string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
WITH expired AS (
  DELETE FROM transfer_velocity_events
  WHERE tenant_id = $1 AND expires_at <= $2
  RETURNING tenant_id, actor_subject_id, source_account_id, amount_minor
), deductions AS (
  SELECT tenant_id, 'tenant'::text AS dimension_type,
         tenant_id::text AS dimension_reference, SUM(amount_minor) AS amount_minor
  FROM expired GROUP BY tenant_id
  UNION ALL
  SELECT tenant_id, 'actor', actor_subject_id, SUM(amount_minor)
  FROM expired GROUP BY tenant_id, actor_subject_id
  UNION ALL
  SELECT tenant_id, 'source', source_account_id::text, SUM(amount_minor)
  FROM expired GROUP BY tenant_id, source_account_id
)
UPDATE transfer_velocity_totals AS totals
SET total_minor = totals.total_minor - deductions.amount_minor,
    updated_at = $2
FROM deductions
WHERE totals.tenant_id = deductions.tenant_id
  AND totals.dimension_type = deductions.dimension_type
  AND totals.dimension_reference = deductions.dimension_reference`, tenantID, now)
	return wrap("prune expired transfer velocity", err)
}

func loadVelocityTotals(ctx context.Context, tx *sql.Tx, command transfers.Command) (velocityTotals, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT dimension_type, total_minor
FROM transfer_velocity_totals
WHERE tenant_id = $1::uuid AND (
  (dimension_type = 'tenant' AND dimension_reference = $1::uuid::text) OR
  (dimension_type = 'actor' AND dimension_reference = $2) OR
  (dimension_type = 'source' AND dimension_reference = $3)
)
FOR UPDATE`, command.TenantID, command.ActorSubjectID, command.DebitAccountID)
	if err != nil {
		return velocityTotals{}, fmt.Errorf("load transfer velocity totals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var totals velocityTotals
	count := 0
	for rows.Next() {
		var dimension string
		var amount int64
		if err := rows.Scan(&dimension, &amount); err != nil {
			return velocityTotals{}, fmt.Errorf("scan transfer velocity total: %w", err)
		}
		switch dimension {
		case "actor":
			totals.actor = amount
		case "source":
			totals.source = amount
		case "tenant":
			totals.tenant = amount
		default:
			return velocityTotals{}, fmt.Errorf("unknown transfer velocity dimension %q", dimension)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return velocityTotals{}, fmt.Errorf("iterate transfer velocity totals: %w", err)
	}
	if count != 3 {
		return velocityTotals{}, fmt.Errorf("load transfer velocity totals: got %d dimensions, want 3", count)
	}
	return totals, nil
}

func recordTransferVelocity(ctx context.Context, tx *sql.Tx, transferID string) error {
	result, err := tx.ExecContext(ctx, `
WITH velocity AS (
INSERT INTO transfer_velocity_events (
  transfer_id, tenant_id, actor_subject_id, source_account_id,
  amount_minor, occurred_at, expires_at
)
SELECT id, tenant_id, actor_subject_id, debit_account_id,
       amount_minor, completed_at, completed_at + INTERVAL '24 hours'
FROM transfers
WHERE id = $1 AND status = 'posted'
RETURNING tenant_id, actor_subject_id, source_account_id, amount_minor, occurred_at
), dimensions AS (
  SELECT tenant_id, 'tenant'::text AS dimension_type,
         tenant_id::text AS dimension_reference, amount_minor, occurred_at
  FROM velocity
  UNION ALL
  SELECT tenant_id, 'actor', actor_subject_id, amount_minor, occurred_at
  FROM velocity
  UNION ALL
  SELECT tenant_id, 'source', source_account_id::text, amount_minor, occurred_at
  FROM velocity
)
INSERT INTO transfer_velocity_totals (
  tenant_id, dimension_type, dimension_reference, total_minor, updated_at
)
SELECT tenant_id, dimension_type, dimension_reference, amount_minor, occurred_at
FROM dimensions
ON CONFLICT (tenant_id, dimension_type, dimension_reference)
DO UPDATE SET total_minor = transfer_velocity_totals.total_minor + EXCLUDED.total_minor,
		updated_at = EXCLUDED.updated_at`, transferID)
	if err != nil {
		return fmt.Errorf("record transfer velocity: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect transfer velocity record: %w", err)
	}
	if rows != 3 {
		return fmt.Errorf("record transfer velocity: changed %d rows, want 3", rows)
	}
	return nil
}
