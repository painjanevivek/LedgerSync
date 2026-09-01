package identity

import (
	"container/heap"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	BFFActorScope                 = "bff:act-as-user"
	DefaultActorAssertionIssuer   = "ledgersync-bff"
	DefaultActorAssertionAudience = "ledgersync-private-api"
	DefaultActorAssertionKeyID    = "current"
	maxActorAssertionRoles        = 16
	maxActorAssertionScopes       = 32
)

type ActorAssertionKey struct {
	ID     string
	Secret []byte
}

type ReplayGuard interface {
	Use(context.Context, string, time.Time) error
}

type ActorAssertionConfig struct {
	Issuer      string
	Audience    string
	CurrentKey  ActorAssertionKey
	PreviousKey *ActorAssertionKey
	MaxLifetime time.Duration
	ClockSkew   time.Duration
	ReplayGuard ReplayGuard
}

// RequestAuthenticator accepts delegated user context only after the caller
// authenticates as the dedicated BFF workload identity. The assertion has its
// own issuer/audience/key/lifetime contract, is bound to the workload's tenant,
// and cannot grant unknown scopes.
type RequestAuthenticator struct {
	provider Provider
	config   ActorAssertionConfig
	now      func() time.Time
}

func NewRequestAuthenticator(provider Provider, assertionSecret string) (*RequestAuthenticator, error) {
	return NewRequestAuthenticatorWithConfig(provider, ActorAssertionConfig{
		Issuer:      DefaultActorAssertionIssuer,
		Audience:    DefaultActorAssertionAudience,
		CurrentKey:  ActorAssertionKey{ID: DefaultActorAssertionKeyID, Secret: []byte(assertionSecret)},
		MaxLifetime: time.Minute,
		ClockSkew:   5 * time.Second,
		ReplayGuard: NewMemoryReplayGuard(10_000),
	})
}

func NewRequestAuthenticatorWithConfig(provider Provider, config ActorAssertionConfig) (*RequestAuthenticator, error) {
	config.Issuer = strings.TrimSpace(config.Issuer)
	config.Audience = strings.TrimSpace(config.Audience)
	config.CurrentKey.ID = strings.TrimSpace(config.CurrentKey.ID)
	if provider == nil || config.Issuer == "" || config.Audience == "" || config.CurrentKey.ID == "" || len(config.CurrentKey.Secret) < 32 {
		return nil, ErrUnauthenticated
	}
	if config.PreviousKey != nil && (strings.TrimSpace(config.PreviousKey.ID) == "" || len(config.PreviousKey.Secret) < 32 || config.PreviousKey.ID == config.CurrentKey.ID) {
		return nil, ErrUnauthenticated
	}
	if config.MaxLifetime <= 0 || config.MaxLifetime > 2*time.Minute || config.ClockSkew < 0 || config.ClockSkew > 30*time.Second || config.ReplayGuard == nil {
		return nil, ErrUnauthenticated
	}
	return &RequestAuthenticator{provider: provider, config: config, now: time.Now}, nil
}

func (a *RequestAuthenticator) Authenticate(ctx context.Context, credential, assertion string) (Principal, error) {
	principal, err := a.provider.Authenticate(ctx, credential)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	if assertion == "" {
		if principal.HasScope(BFFActorScope) {
			return Principal{}, ErrUnauthenticated
		}
		return principal, nil
	}
	if !principal.HasScope(BFFActorScope) {
		return Principal{}, ErrUnauthenticated
	}
	payload, err := verifyActorAssertion(assertion, a.config, a.now())
	if err != nil || !allAllowed(payload.Roles, allowedRoles) || !allAllowed(payload.Scopes, allowedScopes) {
		return Principal{}, ErrUnauthenticated
	}
	if payload.TenantID != principal.TenantID {
		return Principal{}, ErrUnauthenticated
	}
	if err := a.config.ReplayGuard.Use(ctx, payload.AssertionID, time.Unix(payload.ExpiresAt, 0)); err != nil {
		if errors.Is(err, errAssertionReplay) {
			return Principal{}, ErrUnauthenticated
		}
		return Principal{}, ErrAuthenticationUnavailable
	}
	actorPrincipal := Principal{SubjectID: payload.SubjectID, TenantID: payload.TenantID, Roles: allowedSet(payload.Roles, allowedRoles), Scopes: allowedSet(payload.Scopes, allowedScopes)}
	if payload.AuthenticatedAt > 0 {
		actorPrincipal.AuthenticatedAt = time.Unix(payload.AuthenticatedAt, 0).UTC()
	}
	return actorPrincipal, nil
}

type actorAssertionPayload struct {
	Issuer          string   `json:"iss"`
	Audience        string   `json:"aud"`
	KeyID           string   `json:"kid"`
	AssertionID     string   `json:"jti"`
	SubjectID       string   `json:"sub"`
	TenantID        string   `json:"tenant_id"`
	Roles           []string `json:"roles,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	IssuedAt        int64    `json:"iat"`
	ExpiresAt       int64    `json:"exp"`
	AuthenticatedAt int64    `json:"authenticated_at,omitempty"`
}

func verifyActorAssertion(raw string, config ActorAssertionConfig, now time.Time) (actorAssertionPayload, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(parts[0]) > 8*1024 || len(parts[1]) > 1024 {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	var payload actorAssertionPayload
	if json.Unmarshal(payloadBytes, &payload) != nil {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	key, ok := assertionKey(config, payload.KeyID)
	if !ok {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payloadBytes)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	nowUnix := now.Unix()
	if strings.TrimSpace(payload.SubjectID) == "" || strings.TrimSpace(payload.TenantID) == "" || strings.TrimSpace(payload.AssertionID) == "" || len(payload.AssertionID) > 128 ||
		payload.Issuer != config.Issuer || payload.Audience != config.Audience || payload.IssuedAt > nowUnix+int64(config.ClockSkew.Seconds()) ||
		payload.ExpiresAt <= nowUnix-int64(config.ClockSkew.Seconds()) || payload.ExpiresAt <= payload.IssuedAt ||
		payload.AuthenticatedAt > nowUnix+int64(config.ClockSkew.Seconds()) ||
		time.Duration(payload.ExpiresAt-payload.IssuedAt)*time.Second > config.MaxLifetime || len(payload.Roles) > maxActorAssertionRoles || len(payload.Scopes) > maxActorAssertionScopes {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	return payload, nil
}

func assertionKey(config ActorAssertionConfig, keyID string) ([]byte, bool) {
	if keyID == config.CurrentKey.ID {
		return config.CurrentKey.Secret, true
	}
	if config.PreviousKey != nil && keyID == config.PreviousKey.ID {
		return config.PreviousKey.Secret, true
	}
	return nil, false
}

func allAllowed(values []string, allowed map[string]struct{}) bool {
	for _, value := range values {
		if value == "" {
			return false
		}
		if _, ok := allowed[value]; !ok {
			return false
		}
	}
	return true
}

var errAssertionReplay = errors.New("actor assertion replayed")

type MemoryReplayGuard struct {
	mu      sync.Mutex
	entries map[string]time.Time
	expiry  replayExpiryHeap
	limit   int
}

type replayExpiry struct {
	assertionID string
	expiresAt   time.Time
}

type replayExpiryHeap []replayExpiry

func (h replayExpiryHeap) Len() int           { return len(h) }
func (h replayExpiryHeap) Less(i, j int) bool { return h[i].expiresAt.Before(h[j].expiresAt) }
func (h replayExpiryHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *replayExpiryHeap) Push(value any)    { *h = append(*h, value.(replayExpiry)) }
func (h *replayExpiryHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}

func NewMemoryReplayGuard(limit int) *MemoryReplayGuard {
	if limit < 1 {
		limit = 10_000
	}
	return &MemoryReplayGuard{entries: make(map[string]time.Time), expiry: make(replayExpiryHeap, 0, limit), limit: limit}
}

func (g *MemoryReplayGuard) Use(_ context.Context, assertionID string, expiresAt time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.entries[assertionID]; exists {
		return errAssertionReplay
	}
	now := time.Now()
	for g.expiry.Len() > 0 && !g.expiry[0].expiresAt.After(now) {
		expired := heap.Pop(&g.expiry).(replayExpiry)
		if current, exists := g.entries[expired.assertionID]; exists && current.Equal(expired.expiresAt) {
			delete(g.entries, expired.assertionID)
		}
	}
	if len(g.entries) >= g.limit {
		return errAssertionReplay
	}
	g.entries[assertionID] = expiresAt
	heap.Push(&g.expiry, replayExpiry{assertionID: assertionID, expiresAt: expiresAt})
	return nil
}
