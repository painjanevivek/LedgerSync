package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
)

type AccountRepository struct{ database *sql.DB }

func NewAccountRepository(database *sql.DB) (*AccountRepository, error) {
	if database == nil {
		return nil, errors.New("account database is required")
	}
	return &AccountRepository{database: database}, nil
}

func (r *AccountRepository) ListOwned(ctx context.Context, tenantID, actorID string) ([]accounts.Summary, error) {
	page, err := r.ListOwnedPage(ctx, tenantID, actorID, accounts.Query{Limit: 100})
	return page.Accounts, err
}

type accountCursor struct {
	CreatedAt time.Time
	ID        string
}

func decodeAccountCursor(raw string) (accountCursor, error) {
	if raw == "" {
		return accountCursor{}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return accountCursor{}, accounts.ErrInvalidQuery
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return accountCursor{}, accounts.ErrInvalidQuery
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil || strings.TrimSpace(parts[1]) == "" {
		return accountCursor{}, accounts.ErrInvalidQuery
	}
	return accountCursor{CreatedAt: createdAt.UTC(), ID: parts[1]}, nil
}

func encodeAccountCursor(createdAt time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func (r *AccountRepository) ListOwnedPage(ctx context.Context, tenantID, actorID string, query accounts.Query) (accounts.Page, error) {
	cursor, err := decodeAccountCursor(query.Cursor)
	if err != nil {
		return accounts.Page{}, err
	}
	search := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query.Search)
	rows, err := r.database.QueryContext(ctx, `
SELECT a.id, a.version, a.currency, a.status, COALESCE(a.display_name, ''), COALESCE(a.category, 'operating'), COALESCE(a.external_reference, ''), b.available_minor, b.ledger_minor, b.balance_version, b.updated_at, a.created_at
FROM accounts a
JOIN account_owners owner ON owner.tenant_id = a.tenant_id AND owner.account_id = a.id
JOIN account_balance_projections b ON b.account_id = a.id
WHERE a.tenant_id = $1 AND owner.subject_id = $2 AND owner.permission IN ('read', 'debit')
	AND ($3='' OR a.status=$3)
	AND ($4='' OR COALESCE(a.category,'operating')=$4)
	AND ($5='' OR lower(COALESCE(a.display_name,'')) LIKE lower($5)||'%' ESCAPE '\' OR lower(COALESCE(a.external_reference,'')) LIKE lower($5)||'%' ESCAPE '\' OR a.id::text=$5)
	AND ($6::timestamptz IS NULL OR (a.created_at,a.id)>($6::timestamptz,$7::uuid))
ORDER BY a.created_at ASC, a.id ASC LIMIT $8`, tenantID, actorID, query.Status, query.Category, search, nullableTime(cursor.CreatedAt), nullableString(cursor.ID), query.Limit+1)
	if err != nil {
		return accounts.Page{}, fmt.Errorf("list owned accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type row struct {
		Summary   accounts.Summary
		CreatedAt time.Time
	}
	result := make([]row, 0, query.Limit+1)
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.Summary.AccountID, &item.Summary.AccountVersion, &item.Summary.Currency, &item.Summary.Status, &item.Summary.DisplayName, &item.Summary.Category, &item.Summary.ExternalReference, &item.Summary.Balance.AvailableMinor, &item.Summary.Balance.LedgerMinor, &item.Summary.Balance.Version, &item.Summary.Balance.AsOf, &item.CreatedAt); err != nil {
			return accounts.Page{}, fmt.Errorf("scan owned account: %w", err)
		}
		item.Summary.Balance.TenantID, item.Summary.Balance.AccountID, item.Summary.Balance.Currency = tenantID, item.Summary.AccountID, item.Summary.Currency
		item.Summary.Balance.AsOf = item.Summary.Balance.AsOf.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return accounts.Page{}, fmt.Errorf("iterate owned accounts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return accounts.Page{}, fmt.Errorf("close owned account rows: %w", err)
	}
	page := accounts.Page{Accounts: make([]accounts.Summary, 0, query.Limit)}
	if len(result) > query.Limit {
		last := result[query.Limit-1]
		page.NextCursor = encodeAccountCursor(last.CreatedAt, last.Summary.AccountID)
		result = result[:query.Limit]
	}
	for _, item := range result {
		page.Accounts = append(page.Accounts, item.Summary)
	}
	return page, nil
}

func (r *AccountRepository) GetOwned(ctx context.Context, tenantID, actorID, accountID string) (accounts.Summary, error) {
	var item accounts.Summary
	err := r.database.QueryRowContext(ctx, `
SELECT a.id,a.version,a.currency,a.status,COALESCE(a.display_name,''),COALESCE(a.category,'operating'),COALESCE(a.external_reference,''),b.available_minor,b.ledger_minor,b.balance_version,b.updated_at
FROM accounts a
JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id
JOIN account_balance_projections b ON b.account_id=a.id
WHERE a.tenant_id=$1 AND a.id=$2 AND owner.subject_id=$3 AND owner.permission IN ('read','debit')`, tenantID, accountID, actorID).Scan(&item.AccountID, &item.AccountVersion, &item.Currency, &item.Status, &item.DisplayName, &item.Category, &item.ExternalReference, &item.Balance.AvailableMinor, &item.Balance.LedgerMinor, &item.Balance.Version, &item.Balance.AsOf)
	if errors.Is(err, sql.ErrNoRows) {
		return item, accounts.ErrAccountNotFound
	}
	if err != nil {
		return item, fmt.Errorf("get owned account: %w", err)
	}
	item.Balance.TenantID, item.Balance.AccountID, item.Balance.Currency = tenantID, item.AccountID, item.Currency
	item.Balance.AsOf = item.Balance.AsOf.UTC()
	rows, err := r.database.QueryContext(ctx, `SELECT id,event_type,COALESCE(actor_subject_id,''),outcome,correlation_id,occurred_at FROM audit_events WHERE tenant_id=$1 AND target_type='account' AND target_id=$2 ORDER BY occurred_at DESC,id DESC LIMIT 25`, tenantID, accountID)
	if err != nil {
		return item, fmt.Errorf("get account audit context: %w", err)
	}
	defer func() { _ = rows.Close() }()
	item.AuditContext = []accounts.AuditEvent{}
	for rows.Next() {
		var event accounts.AuditEvent
		if err := rows.Scan(&event.EventID, &event.EventType, &event.ActorSubjectID, &event.Outcome, &event.CorrelationID, &event.OccurredAt); err != nil {
			return item, fmt.Errorf("scan account audit context: %w", err)
		}
		event.OccurredAt = event.OccurredAt.UTC()
		item.AuditContext = append(item.AuditContext, event)
	}
	if err := rows.Err(); err != nil {
		return item, fmt.Errorf("iterate account audit context: %w", err)
	}
	if err := rows.Close(); err != nil {
		return item, fmt.Errorf("close account audit rows: %w", err)
	}
	return item, nil
}
