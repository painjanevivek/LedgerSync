// Package accounts owns account-read contracts. It refreshes a disposable
// cache, but PostgreSQL projections are the sole authoritative answer.
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
	// SourceCache is retained for compatibility with existing internal callers.
	// Customer-visible balance reads no longer return Redis projections.
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
	primary  Repository
	cache    Cache
	verifier Verifier
	metrics  Metrics
}

type ReaderConfig struct {
	// MaximumWait and PollInterval are retained for source compatibility.
	// Balance reads no longer wait for Redis because PostgreSQL is the financial
	// source of truth.
	MaximumWait, PollInterval time.Duration
	Metrics                   Metrics
}

func NewReader(primary Repository, cache Cache, verifier Verifier, cfg ReaderConfig) (*Reader, error) {
	if primary == nil || cache == nil {
		return nil, errors.New("balance primary repository and cache are required")
	}
	return &Reader{primary: primary, cache: cache, verifier: verifier, metrics: cfg.Metrics}, nil
}

func (r *Reader) Read(ctx context.Context, tenantID, actorID, accountID, rawRequirement string) (Result, error) {
	if tenantID == "" || actorID == "" || accountID == "" {
		return Result{}, errors.New("tenant, actor, and account are required")
	}
	// ReadCurrent performs authorization and reads the projection in one
	// tenant/owner-scoped PostgreSQL query. Redis is deliberately not consulted
	// for the returned value: a syntactically valid forged cache record must not
	// become customer-visible financial truth.
	balance, err := r.primary.ReadCurrent(ctx, tenantID, actorID, accountID)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCurrentBalanceUnavailable, err)
	}
	// Preserve the non-disclosing authorization boundary: only validate a
	// caller-supplied consistency requirement after the scoped query has proved
	// that this actor may read the account.
	minimum, err := r.minimumVersion(tenantID, accountID, rawRequirement)
	if err != nil {
		return Result{}, err
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
