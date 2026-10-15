package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
)

func (r *TransferRepository) PrepareRecovery(ctx context.Context, command transfers.Command) error {
	fingerprint, err := transfers.Fingerprint(transfers.IdempotencyRequest{TenantID: command.TenantID, ActorSubjectID: command.ActorSubjectID,
		Operation: "transfers.create.v1", Key: command.IdempotencyKey, DebitAccountID: command.DebitAccountID, CreditAccountID: command.CreditAccountID, Amount: command.Amount})
	if err != nil || !transfers.CanonicalRequestReference(command.RequestReference) {
		return transfers.ErrInvalidCommand
	}
	err = WithTenantTransaction(ctx, r.database, TenantScope{TenantID: command.TenantID.String(), ActorID: command.ActorSubjectID}, nil, func(tx *sql.Tx) error {
		_, executeErr := tx.ExecContext(ctx, `SELECT public.controlled_prepare_transfer_recovery_v1($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			command.TenantID, command.ActorSubjectID, command.RequestReference, command.IdempotencyKey,
			command.DebitAccountID, command.CreditAccountID, command.Amount.Currency().Code, command.Amount.Minor(), fingerprint[:])
		return executeErr
	})
	if err != nil {
		return classifyControlledTransferError(err)
	}
	return nil
}

// Scope uses the authenticated tenant and original actor plus current account
// authority. Account status and picker pagination do not block outcome discovery.
const recoverySelect = `
 SELECT r.request_reference::text,r.source_account_id::text,r.destination_account_id::text,
 r.currency,r.amount_minor::text,r.created_at,r.idempotency_key,
 CASE WHEN i.state IN ('completed','failed') THEN COALESCE(i.response_body->>'status','unresolved') ELSE 'unresolved' END,
 COALESCE(i.response_body->>'transfer_id','')
 FROM transfer_recovery_intents r LEFT JOIN idempotency_requests i
 ON i.tenant_id=r.tenant_id AND i.actor_subject_id=r.actor_subject_id AND i.operation='transfers.create.v1' AND i.idempotency_key=r.idempotency_key
 WHERE r.tenant_id=$1 AND r.actor_subject_id=$2
 AND EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=r.tenant_id AND subject_id=$2)
 AND EXISTS(SELECT 1 FROM account_owners WHERE tenant_id=r.tenant_id AND account_id=r.source_account_id AND subject_id=$2 AND permission IN ('read','debit'))
 AND EXISTS(SELECT 1 FROM account_credit_permissions WHERE tenant_id=r.tenant_id AND account_id=r.destination_account_id AND subject_id=$2)`

func scanRecovery(row interface{ Scan(...any) error }) (transfers.RecoveryIntent, error) {
	var item transfers.RecoveryIntent
	err := row.Scan(&item.RequestReference, &item.SourceAccountID, &item.DestinationAccountID, &item.Currency, &item.AmountMinor,
		&item.CreatedAt, &item.IdempotencyKey, &item.State, &item.TransferID)
	item.CreatedAt = item.CreatedAt.UTC()
	return item, err
}

func (r *TransferRepository) GetRecovery(ctx context.Context, tenant, actor, reference string) (transfers.RecoveryIntent, error) {
	tx, scopeErr := beginTenantRead(ctx, r.database, TenantScope{TenantID: tenant, ActorID: actor})
	if scopeErr != nil {
		return transfers.RecoveryIntent{}, scopeErr
	}
	defer func() { _ = tx.Rollback() }()
	item, err := scanRecovery(tx.QueryRowContext(ctx, recoverySelect+` AND r.request_reference=$3`, tenant, actor, reference))
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrInvestigationNotFound
	}
	return item, err
}

func (r *TransferRepository) ListRecovery(ctx context.Context, tenant, actor, rawCursor string) (transfers.RecoveryPage, error) {
	tx, scopeErr := beginTenantRead(ctx, r.database, TenantScope{TenantID: tenant, ActorID: actor})
	if scopeErr != nil {
		return transfers.RecoveryPage{}, scopeErr
	}
	defer func() { _ = tx.Rollback() }()
	cursor, err := decodeInvestigationCursor(rawCursor)
	if err != nil || (rawCursor != "" && !transfers.CanonicalRequestReference(cursor.ID)) {
		return transfers.RecoveryPage{}, transfers.ErrInvalidCommand
	}
	rows, err := tx.QueryContext(ctx, recoverySelect+` AND ($3='' OR (r.created_at,r.request_reference)<($4,NULLIF($3,'')::uuid))
 ORDER BY r.created_at DESC,r.request_reference DESC LIMIT 26`, tenant, actor, cursor.ID, cursor.At)
	// NULLIF prevents an empty first-page identifier being cast to UUID.
	if err != nil {
		return transfers.RecoveryPage{}, err
	}
	defer rows.Close()
	page := transfers.RecoveryPage{Requests: []transfers.RecoveryIntent{}}
	for rows.Next() {
		item, err := scanRecovery(rows)
		if err != nil {
			return page, err
		}
		page.Requests = append(page.Requests, item)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Requests) > 25 {
		page.Requests = page.Requests[:25]
		last := page.Requests[24]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.RequestReference))
	}
	return page, nil
}
