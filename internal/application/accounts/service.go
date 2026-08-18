package accounts

import (
	"context"
	"errors"
)

var ErrAccountNotFound = errors.New("account not found or not authorized")

type Summary struct {
	AccountID string
	Currency  string
	Status    string
	Balance   Balance
}

type OwnedAccountRepository interface {
	ListOwned(context.Context, string, string) ([]Summary, error)
}

type Service struct{ repository OwnedAccountRepository }

func NewService(repository OwnedAccountRepository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("owned account repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) ListOwned(ctx context.Context, tenantID, actorID string) ([]Summary, error) {
	if s == nil || tenantID == "" || actorID == "" {
		return nil, ErrAccountNotFound
	}
	return s.repository.ListOwned(ctx, tenantID, actorID)
}
