// Package cache implements disposable balance projections. It never becomes a
// source of truth: customer-visible financial reads are always obtained from
// PostgreSQL, while these records are only disposable projections.
package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

var ErrCacheMiss = errors.New("balance cache miss")

type Balance struct {
	TenantID       string
	AccountID      string
	Currency       string
	AvailableMinor int64
	LedgerMinor    int64
	Version        int64
	AsOf           time.Time
}

type BalanceCache struct {
	client    redis.UniversalClient
	prefix    string
	ttl       time.Duration
	telemetry *observability.Telemetry
}

func NewBalanceCache(client redis.UniversalClient, prefix string, ttl time.Duration, telemetry ...*observability.Telemetry) (*BalanceCache, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if prefix == "" {
		prefix = "ledgersync:balance:v1"
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &BalanceCache{client: client, prefix: prefix, ttl: ttl, telemetry: firstTelemetry(telemetry)}, nil
}

func (c *BalanceCache) Get(ctx context.Context, tenantID, accountID string) (balance Balance, err error) {
	started := time.Now()
	ctx, span := c.start(ctx, "cache.balance.get")
	defer func() { span.End(); c.observe(ctx, "get", started, err) }()
	values, err := c.client.HGetAll(ctx, c.key(tenantID, accountID)).Result()
	if err != nil {
		return Balance{}, fmt.Errorf("read balance cache: %w", err)
	}
	if len(values) == 0 {
		return Balance{}, ErrCacheMiss
	}
	return decodeBalance(values, tenantID, accountID)
}

// PutIfNewer is an atomic monotonic write. The Lua comparison is intentionally
// string based, avoiding Lua floating-point conversion for 64-bit versions.
func (c *BalanceCache) PutIfNewer(ctx context.Context, balance Balance) (written bool, err error) {
	started := time.Now()
	ctx, span := c.start(ctx, "cache.balance.put_if_newer")
	defer func() { span.End(); c.observe(ctx, "put_if_newer", started, err) }()
	if err := validate(balance); err != nil {
		return false, err
	}
	const script = `
local current = redis.call('HGET', KEYS[1], 'version')
local candidate = ARGV[1]
if current then
  if string.len(current) > string.len(candidate) then return 0 end
  if string.len(current) == string.len(candidate) and current >= candidate then return 0 end
end
redis.call('HSET', KEYS[1],
  'version', candidate,
  'currency', ARGV[2],
  'available_minor', ARGV[3],
  'ledger_minor', ARGV[4],
  'as_of', ARGV[5])
redis.call('PEXPIRE', KEYS[1], ARGV[6])
return 1`
	result, err := c.client.Eval(ctx, script, []string{c.key(balance.TenantID, balance.AccountID)},
		strconv.FormatInt(balance.Version, 10), balance.Currency, strconv.FormatInt(balance.AvailableMinor, 10), strconv.FormatInt(balance.LedgerMinor, 10), balance.AsOf.UTC().Format(time.RFC3339Nano), strconv.FormatInt(c.ttl.Milliseconds(), 10)).Int()
	if err != nil {
		return false, fmt.Errorf("write balance cache: %w", err)
	}
	return result == 1, nil
}

func (c *BalanceCache) Delete(ctx context.Context, tenantID, accountID string) (err error) {
	started := time.Now()
	ctx, span := c.start(ctx, "cache.balance.delete")
	defer func() { span.End(); c.observe(ctx, "delete", started, err) }()
	if err := c.client.Del(ctx, c.key(tenantID, accountID)).Err(); err != nil {
		return fmt.Errorf("delete balance cache: %w", err)
	}
	return nil
}

func firstTelemetry(values []*observability.Telemetry) *observability.Telemetry {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func (c *BalanceCache) start(ctx context.Context, name string) (context.Context, trace.Span) {
	if c.telemetry == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return c.telemetry.Start(ctx, name)
}

func (c *BalanceCache) observe(ctx context.Context, operation string, started time.Time, err error) {
	if c.telemetry != nil {
		c.telemetry.ObserveBoundary(ctx, "cache", operation, started, err)
	}
}

func (c *BalanceCache) key(tenantID, accountID string) string {
	return c.prefix + ":" + tenantID + ":" + accountID
}

func decodeBalance(values map[string]string, tenantID, accountID string) (Balance, error) {
	get := func(key string) (string, error) {
		value := strings.TrimSpace(values[key])
		if value == "" {
			return "", fmt.Errorf("balance cache missing %s", key)
		}
		return value, nil
	}
	versionText, err := get("version")
	if err != nil {
		return Balance{}, err
	}
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version < 0 {
		return Balance{}, errors.New("balance cache has invalid version")
	}
	availableText, err := get("available_minor")
	if err != nil {
		return Balance{}, err
	}
	available, err := strconv.ParseInt(availableText, 10, 64)
	if err != nil || available < 0 {
		return Balance{}, errors.New("balance cache has invalid available_minor")
	}
	ledgerText, err := get("ledger_minor")
	if err != nil {
		return Balance{}, err
	}
	ledger, err := strconv.ParseInt(ledgerText, 10, 64)
	if err != nil || ledger < 0 {
		return Balance{}, errors.New("balance cache has invalid ledger_minor")
	}
	currency, err := get("currency")
	if err != nil {
		return Balance{}, err
	}
	asOfText, err := get("as_of")
	if err != nil {
		return Balance{}, err
	}
	asOf, err := time.Parse(time.RFC3339Nano, asOfText)
	if err != nil {
		return Balance{}, errors.New("balance cache has invalid as_of")
	}
	return Balance{TenantID: tenantID, AccountID: accountID, Currency: currency, AvailableMinor: available, LedgerMinor: ledger, Version: version, AsOf: asOf}, nil
}

func validate(balance Balance) error {
	if balance.TenantID == "" || balance.AccountID == "" || len(balance.Currency) != 3 || balance.Version < 0 || balance.AvailableMinor < 0 || balance.LedgerMinor < 0 || balance.AsOf.IsZero() {
		return errors.New("invalid balance cache record")
	}
	return nil
}
