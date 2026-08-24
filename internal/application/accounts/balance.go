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
	Authorize(context.Context, string, string, string) error
	ReadCurrent(context.Context, string, string, string) (Balance, error)
}
type Cache interface {
	Get(context.Context, string, string) (Balance, error)
	Put(context.Context, Balance) (bool, error)
}
type Verifier interface {
	Verify(string) (consistency.Requirement, error)
}
type Metrics interface {
	ObserveCacheHit()
	ObservePrimaryFallback()
	ObserveUnsatisfiedRequirement()
}

type Reader struct {
	primary      Repository
	cache        Cache
	verifier     Verifier
	maximumWait  time.Duration
	pollInterval time.Duration
	metrics      Metrics
}

type ReaderConfig struct {
	MaximumWait, PollInterval time.Duration
	Metrics                   Metrics
}

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
	return &Reader{primary: primary, cache: cache, verifier: verifier, maximumWait: cfg.MaximumWait, pollInterval: cfg.PollInterval, metrics: cfg.Metrics}, nil
}

func (r *Reader) Read(ctx context.Context, tenantID, actorID, accountID, rawRequirement string) (Result, error) {
	if tenantID == "" || actorID == "" || accountID == "" {
		return Result{}, errors.New("tenant, actor, and account are required")
	}
	// Authorization is an invariant of the read operation, not a side effect of
	// selecting PostgreSQL as the data source. Enforce it before a shared cache
	// can disclose an account projection.
	if err := r.primary.Authorize(ctx, tenantID, actorID, accountID); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCurrentBalanceUnavailable, err)
	}
	minimum, err := r.minimumVersion(tenantID, accountID, rawRequirement)
	if err != nil {
		return Result{}, err
	}
	if cached, err := r.cache.Get(ctx, tenantID, accountID); err == nil && cached.Version >= minimum {
		if r.metrics != nil {
			r.metrics.ObserveCacheHit()
		}
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
					if r.metrics != nil {
						r.metrics.ObserveCacheHit()
					}
					return Result{Balance: cached, Source: SourceCache}, nil
				}
			}
		}
	}
fallback:
	if r.metrics != nil {
		r.metrics.ObservePrimaryFallback()
	}
	balance, err := r.primary.ReadCurrent(ctx, tenantID, actorID, accountID)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCurrentBalanceUnavailable, err)
	}
	if balance.Version < minimum {
		if r.metrics != nil {
			r.metrics.ObserveUnsatisfiedRequirement()
		}
		return Result{}, ErrCurrentBalanceUnavailable
	}
	// A primary read is authoritative and can opportunistically restore a
	// disposable cache after Redis loss. Cache failure never changes the answer.
	_, _ = r.cache.Put(ctx, balance)
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
