package cache

import (
	"context"
	"errors"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
)

// AccountAdapter keeps the application use case independent of Redis details.
type AccountAdapter struct{ cache *BalanceCache }

func NewAccountAdapter(cache *BalanceCache) (*AccountAdapter, error) {
	if cache == nil {
		return nil, errors.New("balance cache is required")
	}
	return &AccountAdapter{cache: cache}, nil
}

func (a *AccountAdapter) Get(ctx context.Context, tenantID, accountID string) (accounts.Balance, error) {
	balance, err := a.cache.Get(ctx, tenantID, accountID)
	if err != nil {
		return accounts.Balance{}, err
	}
	return accounts.Balance{TenantID: balance.TenantID, AccountID: balance.AccountID, Currency: balance.Currency, AvailableMinor: balance.AvailableMinor, LedgerMinor: balance.LedgerMinor, Version: balance.Version, AsOf: balance.AsOf}, nil
}

func (a *AccountAdapter) Put(ctx context.Context, balance accounts.Balance) (bool, error) {
	return a.cache.PutIfNewer(ctx, Balance{TenantID: balance.TenantID, AccountID: balance.AccountID, Currency: balance.Currency, AvailableMinor: balance.AvailableMinor, LedgerMinor: balance.LedgerMinor, Version: balance.Version, AsOf: balance.AsOf})
}
