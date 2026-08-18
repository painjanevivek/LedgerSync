// Package reconciliation records whether current PostgreSQL balance projections
// still agree with the latest durable transfer outbox evidence for each account.
package reconciliation

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	StatusMatched  Status = "matched"
	StatusMismatch Status = "mismatch"
)

type Result struct {
	ID, TenantID                       string
	Status                             Status
	CheckedAccountCount, MismatchCount int
	StartedAt, CompletedAt             time.Time
}
type Repository interface {
	Reconcile(context.Context, string, time.Time) (Result, error)
}
type Service struct {
	repository Repository
	clock      func() time.Time
}

func NewService(repository Repository, clock func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, errors.New("reconciliation repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository, clock}, nil
}
func (s *Service) Run(ctx context.Context, tenantID string) (Result, error) {
	if tenantID == "" {
		return Result{}, errors.New("tenant ID is required")
	}
	return s.repository.Reconcile(ctx, tenantID, s.clock().UTC())
}
