package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const BFFActorScope = "bff:act-as-user"

// RequestAuthenticator only accepts a BFF actor assertion after the caller has
// authenticated as the dedicated BFF workload identity with bff:act-as-user.
// This lets the private API make object-authorization decisions on the same
// OIDC-authenticated user represented by the HttpOnly BFF session.
type RequestAuthenticator struct {
	provider     Provider
	assertionKey []byte
	now          func() time.Time
}

func NewRequestAuthenticator(provider Provider, assertionSecret string) (*RequestAuthenticator, error) {
	if provider == nil || len(assertionSecret) < 32 {
		return nil, ErrUnauthenticated
	}
	return &RequestAuthenticator{provider: provider, assertionKey: []byte(assertionSecret), now: time.Now}, nil
}

func (a *RequestAuthenticator) Authenticate(ctx context.Context, credential, assertion string) (Principal, error) {
	principal, err := a.provider.Authenticate(ctx, credential)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	if assertion == "" {
		return principal, nil
	}
	if !principal.HasScope(BFFActorScope) {
		return Principal{}, ErrUnauthenticated
	}
	payload, err := verifyActorAssertion(assertion, a.assertionKey, a.now())
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{SubjectID: payload.SubjectID, TenantID: payload.TenantID, Roles: allowedSet(payload.Roles, allowedRoles), Scopes: allowedSet(payload.Scopes, allowedScopes)}, nil
}

type actorAssertionPayload struct {
	SubjectID string   `json:"sub"`
	TenantID  string   `json:"tenant_id"`
	Roles     []string `json:"roles,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	ExpiresAt int64    `json:"exp"`
}

func verifyActorAssertion(raw string, key []byte, now time.Time) (actorAssertionPayload, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
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
	var payload actorAssertionPayload
	if json.Unmarshal(payloadBytes, &payload) != nil || strings.TrimSpace(payload.SubjectID) == "" || strings.TrimSpace(payload.TenantID) == "" || payload.ExpiresAt <= now.Unix() || payload.ExpiresAt > now.Add(2*time.Minute).Unix() {
		return actorAssertionPayload{}, ErrUnauthenticated
	}
	return payload, nil
}
