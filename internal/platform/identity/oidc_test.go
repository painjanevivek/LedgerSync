package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestAllowedSetDropsUnknownRolesAndScopes(t *testing.T) {
	roles := allowedSet([]string{"tenant:admin", "platform:root"}, allowedRoles)
	if len(roles) != 1 || !contains(roles, "tenant:admin") {
		t.Fatalf("unexpected roles: %#v", roles)
	}
	scopes := allowedSet([]string{"accounts:read", "accounts:all"}, allowedScopes)
	if len(scopes) != 1 || !contains(scopes, "accounts:read") {
		t.Fatalf("unexpected scopes: %#v", scopes)
	}
}
func contains(values map[string]struct{}, key string) bool { _, ok := values[key]; return ok }

type workloadProvider struct{}

func (workloadProvider) Authenticate(context.Context, string) (Principal, error) {
	return Principal{SubjectID: "bff", TenantID: "system", Scopes: map[string]struct{}{BFFActorScope: {}}}, nil
}

func TestBFFActorAssertionRequiresSignedShortLivedActorContext(t *testing.T) {
	key := "this-is-a-phase-five-test-secret-long-enough"
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	payload, _ := json.Marshal(actorAssertionPayload{Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-001", SubjectID: "customer-user", TenantID: "tenant-a", Scopes: []string{"accounts:read"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(payload)
	assertion := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	authenticator, err := NewRequestAuthenticator(workloadProvider{}, key)
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = func() time.Time { return now }
	principal, err := authenticator.Authenticate(context.Background(), "workload-token", assertion)
	if err != nil || principal.SubjectID != "customer-user" || !principal.HasScope("accounts:read") {
		t.Fatalf("expected verified actor context, principal=%#v err=%v", principal, err)
	}
	if _, err := authenticator.Authenticate(context.Background(), "workload-token", assertion+"x"); err == nil {
		t.Fatal("tampered actor assertion was accepted")
	}
	if _, err := authenticator.Authenticate(context.Background(), "workload-token", assertion); err == nil {
		t.Fatal("replayed actor assertion was accepted")
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
		"wrong audience": {Issuer: DefaultActorAssertionIssuer, Audience: "another-api", KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-aud", SubjectID: "user", TenantID: "tenant", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()},
		"unknown scope":  {Issuer: DefaultActorAssertionIssuer, Audience: DefaultActorAssertionAudience, KeyID: DefaultActorAssertionKeyID, AssertionID: "assertion-scope", SubjectID: "user", TenantID: "tenant", Scopes: []string{"platform:root"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()},
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
