package webhookdelivery

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	appdelivery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
)

const maximumCachedKeyCount = 128

type KeyResolver = appdelivery.KeyResolver

// KeyCacheObserver receives only bounded operational dimensions. References,
// key material, tenants and endpoint identifiers must never be emitted.
type KeyCacheObserver = appdelivery.KeyCacheObserver

// StaticKeyResolver is suitable for a worker-only secret-injection adapter.
// It copies keys on construction and resolution so callers cannot mutate the
// process-held key set. API handlers never receive this implementation.
type StaticKeyResolver map[string][]byte

func NewStaticKeyResolver(keys map[string][]byte) (StaticKeyResolver, error) {
	result := make(StaticKeyResolver, len(keys))
	for reference, key := range keys {
		if strings.TrimSpace(reference) == "" || len(key) < 32 {
			return nil, errors.New("webhook signing key references require at least 32 bytes")
		}
		result[reference] = bytes.Clone(key)
	}
	return result, nil
}

func (r StaticKeyResolver) Resolve(_ context.Context, reference string) ([]byte, error) {
	key, ok := r[strings.TrimSpace(reference)]
	if !ok {
		return nil, errors.New("webhook signing key reference is unavailable")
	}
	return bytes.Clone(key), nil
}

// CachedKeyResolver keeps worker-only managed secret reads bounded. It copies
// values at each boundary and expires entries quickly so endpoint key rotation
// can overlap safely without baking signing material into environment values.
type CachedKeyResolver struct {
	upstream KeyResolver
	ttl      time.Duration
	clock    func() time.Time
	mu       sync.Mutex
	entries  map[string]*list.Element
	recency  *list.List
	observer KeyCacheObserver
}

type cachedKey struct {
	reference string
	value     []byte
	expiresAt time.Time
}

func NewCachedKeyResolver(upstream KeyResolver, ttl time.Duration, clock func() time.Time, observers ...KeyCacheObserver) (*CachedKeyResolver, error) {
	if upstream == nil || ttl <= 0 || ttl > 15*time.Minute {
		return nil, errors.New("managed webhook key resolver requires a 1ns..15m cache TTL")
	}
	if len(observers) > 1 {
		return nil, errors.New("managed webhook key resolver accepts at most one observer")
	}
	if clock == nil {
		clock = time.Now
	}
	var observer KeyCacheObserver
	if len(observers) == 1 {
		observer = observers[0]
	}
	return &CachedKeyResolver{
		upstream: upstream,
		ttl:      ttl,
		clock:    clock,
		entries:  make(map[string]*list.Element),
		recency:  list.New(),
		observer: observer,
	}, nil
}

func (r *CachedKeyResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
	started := time.Now()
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, errors.New("webhook signing key reference is required")
	}
	now := r.clock().UTC()
	lockStarted := time.Now()
	r.mu.Lock()
	lockWait := time.Since(lockStarted)
	if element, ok := r.entries[reference]; ok && element.Value.(*cachedKey).expiresAt.After(now) {
		cached := element.Value.(*cachedKey)
		r.recency.MoveToFront(element)
		value := bytes.Clone(cached.value)
		entryCount := len(r.entries)
		r.mu.Unlock()
		r.observeResolve(ctx, "hit", started, lockWait)
		r.observeEntries(ctx, entryCount)
		return value, nil
	}
	expiredOnLookup := false
	if element, ok := r.entries[reference]; ok {
		r.removeLocked(element)
		expiredOnLookup = true
	}
	entryCount := len(r.entries)
	r.mu.Unlock()
	if expiredOnLookup {
		r.observeEviction(ctx, "expired")
	}
	r.observeEntries(ctx, entryCount)

	value, err := r.upstream.Resolve(ctx, reference)
	if err != nil || len(value) < 32 {
		outcome := "upstream_error"
		if ctx.Err() != nil {
			outcome = "cancelled"
		}
		r.observeResolve(ctx, outcome, started, lockWait)
		return nil, errors.New("managed webhook signing key is unavailable")
	}
	value = bytes.Clone(value)

	now = r.clock().UTC()
	lockStarted = time.Now()
	r.mu.Lock()
	lockWait += time.Since(lockStarted)
	if element, ok := r.entries[reference]; ok && element.Value.(*cachedKey).expiresAt.After(now) {
		cached := element.Value.(*cachedKey)
		r.recency.MoveToFront(element)
		result := bytes.Clone(cached.value)
		entryCount = len(r.entries)
		r.mu.Unlock()
		clear(value)
		r.observeResolve(ctx, "concurrent_hit", started, lockWait)
		r.observeEntries(ctx, entryCount)
		return result, nil
	}
	if element, ok := r.entries[reference]; ok {
		r.removeLocked(element)
	}
	expiredEvictions := 0
	for element := r.recency.Back(); element != nil; {
		previous := element.Prev()
		if !element.Value.(*cachedKey).expiresAt.After(now) {
			r.removeLocked(element)
			expiredEvictions++
		}
		element = previous
	}
	capacityEvicted := false
	if len(r.entries) >= maximumCachedKeyCount {
		r.removeLocked(r.recency.Back())
		capacityEvicted = true
	}
	element := r.recency.PushFront(&cachedKey{reference: reference, value: value, expiresAt: now.Add(r.ttl)})
	r.entries[reference] = element
	result := bytes.Clone(value)
	entryCount = len(r.entries)
	r.mu.Unlock()

	for index := 0; index < expiredEvictions; index++ {
		r.observeEviction(ctx, "expired")
	}
	if capacityEvicted {
		r.observeEviction(ctx, "capacity")
	}
	r.observeResolve(ctx, "miss", started, lockWait)
	r.observeEntries(ctx, entryCount)
	return result, nil
}

func (r *CachedKeyResolver) removeLocked(element *list.Element) {
	if element == nil {
		return
	}
	cached := element.Value.(*cachedKey)
	clear(cached.value)
	delete(r.entries, cached.reference)
	r.recency.Remove(element)
}

func (r *CachedKeyResolver) observeResolve(ctx context.Context, outcome string, started time.Time, lockWait time.Duration) {
	if r.observer != nil {
		r.observer.ObserveWebhookKeyCacheResolve(ctx, outcome, time.Since(started), lockWait)
	}
}

func (r *CachedKeyResolver) observeEntries(ctx context.Context, count int) {
	if r.observer != nil {
		r.observer.ObserveWebhookKeyCacheEntries(ctx, count)
	}
}

func (r *CachedKeyResolver) observeEviction(ctx context.Context, reason string) {
	if r.observer != nil {
		r.observer.ObserveWebhookKeyCacheEviction(ctx, reason)
	}
}

func clear(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
