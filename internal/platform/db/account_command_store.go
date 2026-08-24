package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
)

func authorizeTenantActor(ctx context.Context, tx *sql.Tx, tenantID, actorID string) error {
	var authorized bool
	err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM tenants tenant
  WHERE tenant.id=$1
    AND EXISTS (SELECT 1 FROM tenant_subject_roles role WHERE role.tenant_id=tenant.id AND role.subject_id=$2 AND role.role IN ('operator','finance'))
)`, tenantID, actorID).Scan(&authorized)
	if err != nil {
		return fmt.Errorf("authorize account tenant actor: %w", err)
	}
	if !authorized {
		return accounts.ErrAccountNotFound
	}
	return nil
}

func insertAccountAggregate(ctx context.Context, tx *sql.Tx, aggregate accountdomain.Account) error {
	var insertedID string
	err := tx.QueryRowContext(ctx, `
INSERT INTO accounts (id,tenant_id,currency,status,display_name,category,external_reference,version,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)
ON CONFLICT DO NOTHING
RETURNING id`, aggregate.ID, aggregate.TenantID, aggregate.Currency.Code, aggregate.Status, aggregate.DisplayName, aggregate.Category, aggregate.ExternalReference, aggregate.Version, aggregate.CreatedAt).Scan(&insertedID)
	if errors.Is(err, sql.ErrNoRows) {
		return accounts.ErrAccountConflict
	}
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version,updated_at) VALUES ($1,0,0,0,$2)`, aggregate.ID, aggregate.CreatedAt); err != nil {
		return fmt.Errorf("initialize zero account projection: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_opening_balances(account_id,opening_ledger_minor,created_at) VALUES ($1,0,$2)`, aggregate.ID, aggregate.CreatedAt); err != nil {
		return fmt.Errorf("initialize zero account opening baseline: %w", err)
	}
	owner := aggregate.Owners[0]
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_owners(tenant_id,account_id,subject_id,permission,created_at) VALUES ($1,$2,$3,$4,$5)`, aggregate.TenantID, aggregate.ID, owner.SubjectID, owner.Permission, aggregate.CreatedAt); err != nil {
		return fmt.Errorf("create account owner: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id,created_at) VALUES ($1,$2,$3,$4)`, aggregate.TenantID, aggregate.ID, owner.SubjectID, aggregate.CreatedAt); err != nil {
		return fmt.Errorf("create account credit permission: %w", err)
	}
	return nil
}

func lockOwnedAccount(ctx context.Context, tx *sql.Tx, tenantID, actorID, accountID string) (accountdomain.Account, int64, int64, error) {
	var aggregate accountdomain.Account
	var currency string
	var status accountdomain.Status
	var metadata accountdomain.Metadata
	var version, available, ledger int64
	var createdAt, updatedAt time.Time
	var closedAt sql.NullTime
	var permission accountdomain.Permission
	err := tx.QueryRowContext(ctx, `
SELECT a.id,a.tenant_id,a.currency,a.status,a.display_name,a.external_reference,a.category,a.version,a.created_at,a.updated_at,a.closed_at,
       b.available_minor,b.ledger_minor,owner.permission
FROM accounts a
JOIN account_balance_projections b ON b.account_id=a.id
JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$3 AND owner.permission='debit'
WHERE a.tenant_id=$1 AND a.id=$2
FOR UPDATE OF a,b`, tenantID, accountID, actorID).Scan(&aggregate.ID, &aggregate.TenantID, &currency, &status, &metadata.DisplayName, &metadata.ExternalReference, &metadata.Category, &version, &createdAt, &updatedAt, &closedAt, &available, &ledger, &permission)
	if errors.Is(err, sql.ErrNoRows) {
		return accountdomain.Account{}, 0, 0, accounts.ErrAccountNotFound
	}
	if err != nil {
		return accountdomain.Account{}, 0, 0, fmt.Errorf("lock owned account and projection: %w", err)
	}
	aggregate, err = accountdomain.Restore(aggregate.ID, aggregate.TenantID, currency, status, metadata, version, []accountdomain.Owner{{SubjectID: actorID, Permission: permission}}, createdAt, updatedAt, nullableClosedTime(closedAt))
	if err != nil {
		return accountdomain.Account{}, 0, 0, fmt.Errorf("restore locked account: %w", err)
	}
	return aggregate, available, ledger, nil
}

func authoritativeCloseState(ctx context.Context, tx *sql.Tx, accountID string, available, projectedLedger int64) (accountdomain.FinancialState, error) {
	var inFlight bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM transfers WHERE status='pending' AND (debit_account_id=$1 OR credit_account_id=$1))`, accountID).Scan(&inFlight); err != nil {
		return accountdomain.FinancialState{}, fmt.Errorf("check in-flight account transfers: %w", err)
	}
	if inFlight {
		return accountdomain.FinancialState{}, accountdomain.ErrFinancialStateUnavailable
	}
	var authoritative string
	err := tx.QueryRowContext(ctx, `
SELECT (opening.opening_ledger_minor::numeric + COALESCE(SUM(CASE WHEN posting.direction='credit' THEN posting.amount_minor::numeric ELSE -posting.amount_minor::numeric END),0))::text
FROM account_opening_balances opening
LEFT JOIN ledger_postings posting ON posting.account_id=opening.account_id
WHERE opening.account_id=$1
GROUP BY opening.opening_ledger_minor`, accountID).Scan(&authoritative)
	if errors.Is(err, sql.ErrNoRows) {
		return accountdomain.FinancialState{}, accountdomain.ErrFinancialStateUnavailable
	}
	if err != nil {
		return accountdomain.FinancialState{}, fmt.Errorf("read authoritative account ledger: %w", err)
	}
	authoritativeMinor, err := strconv.ParseInt(authoritative, 10, 64)
	if err != nil || authoritativeMinor < 0 {
		return accountdomain.FinancialState{}, accountdomain.ErrFinancialStateUnavailable
	}
	return accountdomain.FinancialState{AvailableMinor: available, LedgerMinor: projectedLedger, Consistent: authoritativeMinor == projectedLedger}, nil
}

func updateAccountRow(ctx context.Context, tx *sql.Tx, aggregate accountdomain.Account, expectedVersion int64) error {
	if _, err := tx.ExecContext(ctx, `SAVEPOINT account_reference_write`); err != nil {
		return fmt.Errorf("create account reference savepoint: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
UPDATE accounts
SET display_name=$4,external_reference=$5,category=$6,status=$7,closed_at=$8,version=$9,updated_at=$10
WHERE tenant_id=$1 AND id=$2 AND version=$3`, aggregate.TenantID, aggregate.ID, expectedVersion, aggregate.DisplayName, aggregate.ExternalReference, aggregate.Category, aggregate.Status, nullableTime(aggregate.ClosedAt), aggregate.Version, aggregate.UpdatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			if _, rollbackErr := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT account_reference_write`); rollbackErr != nil {
				return fmt.Errorf("rollback account reference conflict: %w", rollbackErr)
			}
			if _, releaseErr := tx.ExecContext(ctx, `RELEASE SAVEPOINT account_reference_write`); releaseErr != nil {
				return fmt.Errorf("release account reference savepoint: %w", releaseErr)
			}
			return accounts.ErrAccountConflict
		}
		return fmt.Errorf("update account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT account_reference_write`); err != nil {
		return fmt.Errorf("release account reference savepoint: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read account update result: %w", err)
	}
	if rows != 1 {
		return accounts.ErrVersionConflict
	}
	return nil
}

func insertAccountAudit(ctx context.Context, tx *sql.Tx, envelope commandEnvelope, accountID, eventType, outcome string, metadata map[string]string) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(sanitizeAuditMetadata(metadata))
	if err != nil {
		return fmt.Errorf("marshal account audit metadata: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)
VALUES($1,$2,$3,$4,'account',NULLIF($5,''),$6,$7,$8,$9)`, id, envelope.TenantID, envelope.ActorID, eventType, accountID, outcome, envelope.CorrelationID, encoded, envelope.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert account audit event: %w", err)
	}
	return nil
}

func insertAccountOutbox(ctx context.Context, tx *sql.Tx, envelope commandEnvelope, aggregate accountdomain.Account, eventType string, summary ...map[string]string) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	payloadValues := map[string]string{
		"event_id": id, "event_type": eventType, "aggregate_type": "account", "aggregate_id": aggregate.ID,
		"account_id": aggregate.ID, "currency": aggregate.Currency.Code, "status": string(aggregate.Status),
		"version": strconv.FormatInt(aggregate.Version, 10), "occurred_at": envelope.OccurredAt.Format(time.RFC3339Nano),
	}
	if len(summary) > 0 {
		for key, value := range sanitizeAuditMetadata(summary[0]) {
			payloadValues[key] = value
		}
	}
	payload, err := json.Marshal(payloadValues)
	if err != nil {
		return fmt.Errorf("marshal account outbox event: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO outbox_events(id,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at)
VALUES($1,$2,NULL,$3,'account',$3,$4,$5,$6,$7)`, id, envelope.TenantID, aggregate.ID, eventType, aggregate.Version, payload, envelope.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert account outbox event: %w", err)
	}
	return nil
}

func accountCommandResult(aggregate accountdomain.Account, available, ledger int64) accounts.CommandResult {
	return accounts.CommandResult{
		AccountID: aggregate.ID, TenantID: aggregate.TenantID, Currency: aggregate.Currency.Code, Status: string(aggregate.Status),
		DisplayName: aggregate.DisplayName, Reference: aggregate.ExternalReference, Category: aggregate.Category, Version: strconv.FormatInt(aggregate.Version, 10),
		AvailableMinor: strconv.FormatInt(available, 10), LedgerMinor: strconv.FormatInt(ledger, 10),
		CreatedAt: aggregate.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: aggregate.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func nullableClosedTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}
