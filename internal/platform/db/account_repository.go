package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	rows, err := r.database.QueryContext(ctx, `
SELECT a.id, a.currency, a.status, b.available_minor, b.ledger_minor, b.balance_version, b.updated_at
FROM accounts a
JOIN account_owners owner ON owner.tenant_id = a.tenant_id AND owner.account_id = a.id
JOIN account_balance_projections b ON b.account_id = a.id
WHERE a.tenant_id = $1 AND owner.subject_id = $2 AND owner.permission IN ('read', 'debit')
ORDER BY a.created_at ASC, a.id ASC`, tenantID, actorID)
	if err != nil {
		return nil, fmt.Errorf("list owned accounts: %w", err)
	}
	defer rows.Close()
	var result []accounts.Summary
	for rows.Next() {
		var item accounts.Summary
		if err := rows.Scan(&item.AccountID, &item.Currency, &item.Status, &item.Balance.AvailableMinor, &item.Balance.LedgerMinor, &item.Balance.Version, &item.Balance.AsOf); err != nil {
			return nil, fmt.Errorf("scan owned account: %w", err)
		}
		item.Balance.TenantID, item.Balance.AccountID, item.Balance.Currency = tenantID, item.AccountID, item.Currency
		item.Balance.AsOf = item.Balance.AsOf.UTC()
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate owned accounts: %w", err)
	}
	return result, nil
}
