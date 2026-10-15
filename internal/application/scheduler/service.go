// Package scheduler selects a lease owner through PostgreSQL. It has no
// financial command authority and does not implement a consensus algorithm.
package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/lease"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
)

type Store interface {
	Acquire(context.Context, string, string, time.Duration) (int64, error)
	Renew(context.Context, string, string, int64, time.Duration) error
	Claim(context.Context, string, string, int64, time.Duration) (string, error)
	Complete(context.Context, string, string, int64, string, reconciliation.Result) error
	Release(context.Context, string, string, int64) error
}
type Runner interface {
	Run(context.Context, string) (reconciliation.Result, error)
}
type Service struct {
	store              Store
	runner             Runner
	owner              string
	duration, interval time.Duration
}

func New(store Store, runner Runner, owner string, duration, interval time.Duration) (*Service, error) {
	if store == nil || runner == nil || owner == "" || len(owner) > 128 || duration < time.Second || duration > time.Minute || interval < 15*time.Second || interval > 24*time.Hour {
		return nil, errors.New("invalid scheduler configuration")
	}
	return &Service{store, runner, owner, duration, interval}, nil
}
func (s *Service) Tick(ctx context.Context, tenant string) error {
	token, err := s.store.Acquire(ctx, tenant, s.owner, s.duration)
	if err != nil || token == 0 {
		return err
	}
	err = lease.Run(ctx, s.duration, func(renewCtx context.Context) error {
		return s.store.Renew(renewCtx, tenant, s.owner, token, s.duration)
	}, func(workCtx context.Context) error {
		job, err := s.store.Claim(workCtx, tenant, s.owner, token, s.interval)
		if err != nil || job == "" {
			return err
		}
		result, err := s.runner.Run(workCtx, tenant)
		if err != nil {
			return err
		}
		return s.store.Complete(workCtx, tenant, s.owner, token, job, result)
	})
	if err != nil {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = s.store.Release(releaseCtx, tenant, s.owner, token)
	}
	return err
}
