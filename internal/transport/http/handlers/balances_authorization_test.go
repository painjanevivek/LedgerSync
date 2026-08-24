package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

func TestBalanceHandlerReturnsSameNonDisclosingDenialForWarmAndColdCache(t *testing.T) {
	tests := []struct {
		name  string
		entry accounts.Balance
		err   error
	}{
		{
			name: "warm cache",
			entry: accounts.Balance{
				TenantID:       "tenant-a",
				AccountID:      "account-owned-by-another-actor",
				Currency:       "INR",
				AvailableMinor: 900_00,
				LedgerMinor:    900_00,
				Version:        7,
				AsOf:           time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
			},
		},
		{name: "cache miss", err: cache.ErrCacheMiss},
	}

	var expectedBody string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cached := &authorizationTestCache{entry: test.entry, err: test.err}
			reader, err := accounts.NewReader(denyingBalanceRepository{}, cached, nil, accounts.ReaderConfig{})
			if err != nil {
				t.Fatal(err)
			}
			handler := NewBalanceHandler(reader, balanceAuthorizationProvider{})
			request := httptest.NewRequest(http.MethodGet, "/api/accounts/account-owned-by-another-actor/balance", nil)
			request.Header.Set("Authorization", "Bearer valid-least-privileged-token")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want non-disclosing 404; body=%s", response.Code, response.Body.String())
			}
			if cached.getCalls != 0 {
				t.Fatalf("shared cache was read %d times before account authorization", cached.getCalls)
			}
			if expectedBody == "" {
				expectedBody = response.Body.String()
			} else if response.Body.String() != expectedBody {
				t.Fatalf("response body differs by cache state: got %q want %q", response.Body.String(), expectedBody)
			}
		})
	}
}

type denyingBalanceRepository struct{}

func (denyingBalanceRepository) Authorize(context.Context, string, string, string) error {
	return db.ErrBalanceNotAuthorized
}

func (denyingBalanceRepository) ReadCurrent(context.Context, string, string, string) (accounts.Balance, error) {
	return accounts.Balance{}, db.ErrBalanceNotAuthorized
}

type authorizationTestCache struct {
	entry    accounts.Balance
	err      error
	getCalls int
}

func (c *authorizationTestCache) Get(context.Context, string, string) (accounts.Balance, error) {
	c.getCalls++
	return c.entry, c.err
}

func (*authorizationTestCache) Put(context.Context, accounts.Balance) (bool, error) {
	return false, nil
}

type balanceAuthorizationProvider struct{}

func (balanceAuthorizationProvider) Authenticate(context.Context, string) (identity.Principal, error) {
	return identity.Principal{
		SubjectID: "least-privileged-actor",
		TenantID:  "tenant-a",
		Scopes:    map[string]struct{}{"accounts:read": {}},
	}, nil
}

var _ accounts.Repository = denyingBalanceRepository{}
var _ accounts.Cache = (*authorizationTestCache)(nil)
var _ identity.Provider = balanceAuthorizationProvider{}
