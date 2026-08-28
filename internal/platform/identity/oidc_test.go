package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAllowedSetDropsUnknownRolesAndScopes(t *testing.T) {
	roles := allowedSet([]string{"tenant:admin", "platform:root"}, allowedRoles)
	if len(roles) != 1 || !contains(roles, "tenant:admin") {
		t.Fatalf("unexpected roles: %#v", roles)
	}
	scopes := allowedSet([]string{"accounts:read", "accounts:write", "reconciliation:write", "local:read", "local:write", "events:read", "developer:read", "recovery:read", "exports:read", "explainability:read", "accounts:all"}, allowedScopes)
	if len(scopes) != 10 || !contains(scopes, "accounts:read") || !contains(scopes, "accounts:write") || !contains(scopes, "reconciliation:write") || !contains(scopes, "local:read") || !contains(scopes, "local:write") || !contains(scopes, "events:read") || !contains(scopes, "developer:read") || !contains(scopes, "recovery:read") || !contains(scopes, "exports:read") || !contains(scopes, "explainability:read") {
		t.Fatalf("unexpected scopes: %#v", scopes)
	}
}

func TestCognitoAccessTokenRequiresPurposeAudienceAndServerClientMapping(t *testing.T) {
	valid := accessTokenClaims{
		ClientID: "partner-client",
		TokenUse: "access",
		Audience: []string{"https://api.ledgersync.example"},
		Scope:    "accounts:read accounts:write transfers:write reconciliation:write unknown:scope",
	}
	principal, err := principalFromAccessTokenClaims(valid, "https://api.ledgersync.example", map[string]string{"partner-client": "tenant-a"})
	if err != nil {
		t.Fatal(err)
	}
	if principal.SubjectID != "oauth-client:partner-client" || principal.TenantID != "tenant-a" || !principal.HasScope("accounts:read") || !principal.HasScope("accounts:write") || !principal.HasScope("transfers:write") || !principal.HasScope("reconciliation:write") || principal.HasScope("unknown:scope") {
		t.Fatalf("unexpected workload principal: %#v", principal)
	}

	for name, claims := range map[string]accessTokenClaims{
		"id token":          {ClientID: valid.ClientID, TokenUse: "id", Audience: valid.Audience, Scope: valid.Scope},
		"wrong audience":    {ClientID: valid.ClientID, TokenUse: valid.TokenUse, Audience: []string{"another-api"}, Scope: valid.Scope},
		"unmapped client":   {ClientID: "unknown", TokenUse: valid.TokenUse, Audience: valid.Audience, Scope: valid.Scope},
		"missing client id": {TokenUse: valid.TokenUse, Audience: valid.Audience, Scope: valid.Scope},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := principalFromAccessTokenClaims(claims, "https://api.ledgersync.example", map[string]string{"partner-client": "tenant-a"}); err == nil {
				t.Fatal("invalid Cognito access token claims were accepted")
			}
		})
	}
}
func contains(values map[string]struct{}, key string) bool { _, ok := values[key]; return ok }

type workloadProvider struct{}

func (workloadProvider) Authenticate(context.Context, string) (Principal, error) {
	return Principal{SubjectID: "bff", TenantID: "tenant-a", Scopes: map[string]struct{}{BFFActorScope: {}}}, nil
}

func TestBFFActorAssertionRequiresSignedShortLivedActorContext(t *testing.T) {
	key := "this-is-a-phase-five-test-secret-long-enough"
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	payload, _ := json.Marshal(actorAssertionPayload{Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-001", SubjectID: "customer-user", TenantID: "tenant-a", Scopes: []string{"accounts:read", "accounts:write"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(payload)
	assertion := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	authenticator, err := NewRequestAuthenticator(workloadProvider{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = func() time.Time { return now }
	principal, err := authenticator.Authenticate(context.Background(), "workload-token", assertion)
	if err != nil || principal.SubjectID != "customer-user" || !principal.HasScope("accounts:read") || !principal.HasScope("accounts:write") {
		t.Fatalf("expected verified actor context, principal=%#v err=%v", principal, err)
	}
	if principal.TenantID != "tenant-a" {
		t.Fatalf("expected workload-bound tenant tenant-a, got %q", principal.TenantID)
	}
	if _, err := authenticator.Authenticate(context.Background(), "workload-token", assertion+"x"); err == nil {
		t.Fatal("tampered actor assertion was accepted")
	}
	if _, err := authenticator.Authenticate(context.Background(), "workload-token", assertion); err == nil {
		t.Fatal("replayed actor assertion was accepted")
	}
}

func TestBFFActorAssertionRejectsTenantDifferentFromWorkload(t *testing.T) {
	key := "this-is-a-phase-five-test-secret-long-enough"
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(actorAssertionPayload{Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-cross-tenant", SubjectID: "customer-user", TenantID: "tenant-b", Scopes: []string{"accounts:read"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(payload)
	assertion := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	authenticator, err := NewRequestAuthenticator(workloadProvider{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = func() time.Time { return now }

	_, err = authenticator.Authenticate(context.Background(), "workload-token", assertion)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected tenant-a workload to reject correctly signed tenant-b assertion, got %v", err)
	}
}

func TestBFFWorkloadCannotActWithoutActorAssertion(t *testing.T) {
	authenticator, err := NewRequestAuthenticator(workloadProvider{}, "this-is-a-phase-one-test-secret-long-enough")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticator.Authenticate(context.Background(), "workload-token", ""); err == nil {
		t.Fatal("BFF workload token was accepted without delegated actor context")
	}
}

func TestBFFActorAssertionRejectsWrongAudienceAndOverScopedActor(t *testing.T) {
	key := "this-is-a-phase-five-test-secret-long-enough"
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	authenticator, err := NewRequestAuthenticator(workloadProvider{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = func() time.Time { return now }

	for name, payload := range map[string]actorAssertionPayload{
		"wrong audience": {Issuer: DefaultActorAssertionIssuer, Audience: "another-api", KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-aud", SubjectID: "user", TenantID: "tenant-a", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()},
		"unknown scope":  {Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-scope", SubjectID: "user", TenantID: "tenant-a", Scopes: []string{"platform:root"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, _ := json.Marshal(payload)
			mac := hmac.New(sha256.New, []byte(key))
			_, _ = mac.Write(encoded)
			assertion := base64.RawURLEncoding.EncodeToString(encoded) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
			if _, err := authenticator.Authenticate(context.Background(), "workload-token", assertion); err == nil {
				t.Fatal("invalid assertion was accepted")
			}
		})
	}
}

func TestBFFActorAssertionAcceptsPreviousKeyDuringRotation(t *testing.T) {
	current := ActorAssertionKey{ID: "current", Secret: []byte("current-actor-assertion-secret-long-enough")}
	previous := ActorAssertionKey{ID: "previous", Secret: []byte("previous-actor-assertion-secret-long-enough")}
	now := time.Now().UTC()
	authenticator, err := NewRequestAuthenticatorWithConfig(workloadProvider{}, ActorAssertionConfig{Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, CurrentKey: current, PreviousKey: &previous, MaxLifetime: time.Minute, ClockSkew: 5 * time.Second, ReplayGuard: NewMemoryReplayGuard(10)})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(actorAssertionPayload{Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, KeyID: previous.ID, AssertionID: "rotation-assertion", SubjectID: "customer-user", TenantID: "tenant-a", Scopes: []string{"accounts:read"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	mac := hmac.New(sha256.New, previous.Secret)
	_, _ = mac.Write(payload)
	assertion := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if _, err := authenticator.Authenticate(context.Background(), "workload-token", assertion); err != nil {
		t.Fatalf("previous key rejected during overlap: %v", err)
	}
}

func TestMemoryReplayGuardExpiresIncrementallyAndEnforcesCapacity(t *testing.T) {
	guard := NewMemoryReplayGuard(2)
	now := time.Now()
	if err := guard.Use(context.Background(), "first", now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := guard.Use(context.Background(), "second", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := guard.Use(context.Background(), "third", now.Add(time.Minute)); err != nil {
		t.Fatalf("expired entry was not reclaimed: %v", err)
	}
	if err := guard.Use(context.Background(), "fourth", now.Add(time.Minute)); err == nil {
		t.Fatal("replay guard exceeded its bounded capacity")
	}
	if err := guard.Use(context.Background(), "second", now.Add(time.Minute)); err == nil {
		t.Fatal("duplicate assertion was accepted")
	}
}
