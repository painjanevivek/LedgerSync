// Package accounts owns account-read contracts. It asks a disposable cache for
// speed but treats PostgreSQL projections as the sole authoritative answer.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
)

var ErrCurrentBalanceUnavailable = errors.New("current balance is temporarily unavailable")

type Balance struct {
	TenantID       string
	AccountID      string
	Currency       string
	AvailableMinor int64
	LedgerMinor    int64
	Version        int64
	AsOf           time.Time
}

type Source string

const (
	SourceCache   Source = "cache"
	SourcePrimary Source = "primary"
)

type Result struct {
	Balance Balance
	Source  Source
}

type Repository interface {
	ReadCurrent(context.Context, string, string, string) (Balance, error)
}
type Cache interface {
	Get(context.Context, string, string) (Balance, error)
}
type Verifier interface {
	Verify(string) (consistency.Requirement, error)
}

type Reader struct {
	primary      Repository
	cache        Cache
	verifier     Verifier
	maximumWait  time.Duration
	pollInterval time.Duration
}

type ReaderConfig struct{ MaximumWait, PollInterval time.Duration }

func NewReader(primary Repository, cache Cache, verifier Verifier, cfg ReaderConfig) (*Reader, error) {
	if primary == nil || cache == nil {
		return nil, errors.New("balance primary repository and cache are required")
	}
	if cfg.MaximumWait <= 0 {
		cfg.MaximumWait = 150 * time.Millisecond
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 15 * time.Millisecond
	}
	return &Reader{primary: primary, cache: cache, verifier: verifier, maximumWait: cfg.MaximumWait, pollInterval: cfg.PollInterval}, nil
}

func (r *Reader) Read(ctx context.Context, tenantID, actorID, accountID, rawRequirement string) (Result, error) {
	if tenantID == "" || actorID == "" || accountID == "" {
		return Result{}, errors.New("tenant, actor, and account are required")
	}
	minimum, err := r.minimumVersion(tenantID, accountID, rawRequirement)
	if err != nil {
		return Result{}, err
	}
	if cached, err := r.cache.Get(ctx, tenantID, accountID); err == nil && cached.Version >= minimum {
		return Result{Balance: cached, Source: SourceCache}, nil
	}
	if minimum > 0 {
		deadline := time.NewTimer(r.maximumWait)
		defer deadline.Stop()
		ticker := time.NewTicker(r.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-deadline.C:
				goto fallback
			case <-ticker.C:
				if cached, err := r.cache.Get(ctx, tenantID, accountID); err == nil && cached.Version >= minimum {
					return Result{Balance: cached, Source: SourceCache}, nil
				}
			}
		}
	}
fallback:
	balance, err := r.primary.ReadCurrent(ctx, tenantID, actorID, accountID)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrCurrentBalanceUnavailable, err)
	}
	if balance.Version < minimum {
		return Result{}, ErrCurrentBalanceUnavailable
	}
	return Result{Balance: balance, Source: SourcePrimary}, nil
}

func (r *Reader) minimumVersion(tenantID, accountID, raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	if r.verifier == nil {
		return 0, errors.New("consistency requirement is not configured")
	}
	requirement, err := r.verifier.Verify(raw)
	if err != nil {
		return 0, err
	}
	if requirement.TenantID != tenantID || requirement.AccountID != accountID {
		return 0, errors.New("consistency requirement does not match account")
	}
	return requirement.MinimumVersion, nil
}
