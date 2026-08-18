// Package cache implements disposable balance projections. It never becomes a
// source of truth: every record carries the PostgreSQL projection version and
// callers must fall back to the primary database when freshness is required.
package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
	client redis.UniversalClient
	prefix string
	ttl    time.Duration
}

func NewBalanceCache(client redis.UniversalClient, prefix string, ttl time.Duration) (*BalanceCache, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if prefix == "" {
		prefix = "ledgersync:balance:v1"
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &BalanceCache{client: client, prefix: prefix, ttl: ttl}, nil
}

func (c *BalanceCache) Get(ctx context.Context, tenantID, accountID string) (Balance, error) {
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
func (c *BalanceCache) PutIfNewer(ctx context.Context, balance Balance) (bool, error) {
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

func (c *BalanceCache) Delete(ctx context.Context, tenantID, accountID string) error {
	if err := c.client.Del(ctx, c.key(tenantID, accountID)).Err(); err != nil {
		return fmt.Errorf("delete balance cache: %w", err)
	}
	return nil
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
