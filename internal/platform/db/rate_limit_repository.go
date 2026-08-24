package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type RateLimitDecision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type RateLimitRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewRateLimitRepository(database *sql.DB, clock func() time.Time) (*RateLimitRepository, error) {
	if database == nil {
		return nil, errors.New("rate limit database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &RateLimitRepository{database: database, clock: clock}, nil
}

// Consume uses one PostgreSQL upsert, so every API replica observes the same
// tenant/principal/route window. State is operational and may be discarded;
// it is never used as financial evidence.
func (r *RateLimitRepository) Consume(ctx context.Context, tenantID, principalID, route string, limit int, window time.Duration) (RateLimitDecision, error) {
	if tenantID == "" || principalID == "" || route == "" || limit < 1 || window < time.Second {
		return RateLimitDecision{}, errors.New("valid rate limit dimensions are required")
	}
	now := r.clock().UTC()
	windowSeconds := int64(window / time.Second)
	started := time.Unix((now.Unix()/windowSeconds)*windowSeconds, 0).UTC()
	principalHash := sha256.Sum256([]byte(principalID))
	var count int
	err := r.database.QueryRowContext(ctx, `
INSERT INTO api_rate_limit_windows (tenant_id,principal_hash,route_key,window_started_at,request_count)
VALUES ($1,$2,$3,$4,1)
ON CONFLICT (tenant_id,principal_hash,route_key,window_started_at)
DO UPDATE SET request_count=api_rate_limit_windows.request_count+1
RETURNING request_count`, tenantID, principalHash[:], route, started).Scan(&count)
	if err != nil {
		return RateLimitDecision{}, fmt.Errorf("consume rate limit: %w", err)
	}
	if count <= limit {
		return RateLimitDecision{Allowed: true}, nil
	}
	retry := started.Add(window).Sub(now)
	if retry < time.Second {
		retry = time.Second
	}
	return RateLimitDecision{RetryAfter: retry}, nil
}
