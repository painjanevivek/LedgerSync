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
