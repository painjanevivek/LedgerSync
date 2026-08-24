package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
)

var ErrBalanceNotAuthorized = errors.New("balance account not found or not authorized")

type BalanceRepository struct{ database *sql.DB }

func NewBalanceRepository(database *sql.DB) (*BalanceRepository, error) {
	if database == nil {
		return nil, errors.New("balance database is required")
	}
	return &BalanceRepository{database: database}, nil
}

// Authorize proves account visibility before any shared cache value can be
// returned. It uses the same non-disclosing result for absent and inaccessible
// accounts.
func (r *BalanceRepository) Authorize(ctx context.Context, tenantID, actorID, accountID string) error {
	var authorized bool
	err := r.database.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM account_owners
  WHERE tenant_id = $1 AND account_id = $2 AND subject_id = $3
    AND permission IN ('read', 'debit')
)`, tenantID, accountID, actorID).Scan(&authorized)
	if err != nil {
		return fmt.Errorf("authorize balance read: %w", err)
	}
	if !authorized {
		return ErrBalanceNotAuthorized
	}
	return nil
}

// ReadCurrent checks ownership in the same query as the authoritative
// projection. It intentionally returns one safe denial for absent/inaccessible
// accounts so callers do not disclose another account's existence.
func (r *BalanceRepository) ReadCurrent(ctx context.Context, tenantID, actorID, accountID string) (accounts.Balance, error) {
	var balance accounts.Balance
	var updatedAt time.Time
	err := r.database.QueryRowContext(ctx, `
SELECT a.currency, b.available_minor, b.ledger_minor, b.balance_version, b.updated_at
FROM accounts AS a
JOIN account_balance_projections AS b ON b.account_id = a.id
JOIN account_owners AS owner ON owner.tenant_id = a.tenant_id AND owner.account_id = a.id
WHERE a.tenant_id = $1 AND a.id = $2 AND owner.subject_id = $3
  AND owner.permission IN ('read', 'debit')`, tenantID, accountID, actorID).Scan(&balance.Currency, &balance.AvailableMinor, &balance.LedgerMinor, &balance.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return accounts.Balance{}, ErrBalanceNotAuthorized
	}
	if err != nil {
		return accounts.Balance{}, fmt.Errorf("read current balance: %w", err)
	}
	balance.TenantID, balance.AccountID, balance.AsOf = tenantID, accountID, updatedAt.UTC()
	return balance, nil
}

// ListCurrentForTenant is an operational-only projection read for the cache
// rebuild command. It performs no financial write and must not be exposed via
// a customer HTTP route.
func (r *BalanceRepository) ListCurrentForTenant(ctx context.Context, tenantID string) ([]accounts.Balance, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT a.id, a.currency, b.available_minor, b.ledger_minor, b.balance_version, b.updated_at FROM accounts a JOIN account_balance_projections b ON b.account_id = a.id WHERE a.tenant_id = $1 ORDER BY a.id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list current balances: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var balances []accounts.Balance
	for rows.Next() {
		var b accounts.Balance
		if err := rows.Scan(&b.AccountID, &b.Currency, &b.AvailableMinor, &b.LedgerMinor, &b.Version, &b.AsOf); err != nil {
			return nil, fmt.Errorf("scan current balance: %w", err)
		}
		b.TenantID = tenantID
		b.AsOf = b.AsOf.UTC()
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate current balances: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close current balance rows: %w", err)
	}
	return balances, nil
}
