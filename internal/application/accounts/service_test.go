package accounts

import (
	"context"
	"errors"
	"testing"
)

type failingOwnedAccountRepository struct{ err error }

func (r failingOwnedAccountRepository) ListOwnedPage(context.Context, string, string, Query) (Page, error) {
	return Page{}, r.err
}

func (r failingOwnedAccountRepository) GetOwned(context.Context, string, string, string) (Summary, error) {
	return Summary{}, r.err
}

func TestAccountServiceClassifiesRepositoryFailureAsUnavailable(t *testing.T) {
	service, err := NewService(failingOwnedAccountRepository{err: errors.New("database connection refused")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListOwned(context.Background(), "tenant", "actor", Query{}); !errors.Is(err, ErrAccountDirectoryUnavailable) {
		t.Fatalf("list error=%v, want unavailable", err)
	}
	if _, err := service.GetOwned(context.Background(), "tenant", "actor", "account"); !errors.Is(err, ErrAccountDirectoryUnavailable) {
		t.Fatalf("get error=%v, want unavailable", err)
	}
}

func TestAccountServicePreservesSafeRepositoryOutcomes(t *testing.T) {
	service, err := NewService(failingOwnedAccountRepository{err: ErrAccountNotFound})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetOwned(context.Background(), "tenant", "actor", "account"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("get error=%v, want not found", err)
	}
}
