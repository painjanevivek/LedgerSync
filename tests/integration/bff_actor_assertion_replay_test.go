package integration_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type replayWorkloadProvider struct{}

func (replayWorkloadProvider) Authenticate(context.Context, string) (identity.Principal, error) {
	return identity.Principal{SubjectID: "bff", TenantID: testTenantID, Scopes: map[string]struct{}{identity.BFFActorScope: {}}}, nil
}

func TestBFFActorAssertionReplayIsRejectedAcrossAPIReplicas(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	now := time.Now().UTC().Truncate(time.Second)
	firstGuard, err := identity.NewPostgresReplayGuard(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	secondGuard, err := identity.NewPostgresReplayGuard(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("shared-postgres-replay-guard-test-key-long-enough")
	configuration := func(guard identity.ReplayGuard) identity.ActorAssertionConfig {
		return identity.ActorAssertionConfig{Issuer: identity.DefaultActorAssertionIssuer, Audience: identity.DefaultActorAssertionAudience, CurrentKey: identity.ActorAssertionKey{ID: identity.DefaultActorAssertionKeyID, Secret: key}, MaxLifetime: time.Minute, ClockSkew: 5 * time.Second, ReplayGuard: guard}
	}
	first, err := identity.NewRequestAuthenticatorWithConfig(replayWorkloadProvider{}, configuration(firstGuard))
	if err != nil {
		t.Fatal(err)
	}
	second, err := identity.NewRequestAuthenticatorWithConfig(replayWorkloadProvider{}, configuration(secondGuard))
	if err != nil {
		t.Fatal(err)
	}
	assertion := signedReplayAssertion(t, key, now)
	if _, err := first.Authenticate(context.Background(), "workload-token", assertion); err != nil {
		t.Fatalf("first API replica rejected a new assertion: %v", err)
	}
	if _, err := second.Authenticate(context.Background(), "workload-token", assertion); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("second API replica accepted a replay: %v", err)
	}
}

func signedReplayAssertion(t *testing.T, key []byte, now time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"iss": identity.DefaultActorAssertionIssuer, "aud": identity.DefaultActorAssertionAudience,
		"kid": identity.DefaultActorAssertionKeyID, "jti": "cross-replica-assertion-001",
		"sub": "operator", "tenant_id": testTenantID, "scopes": []string{"accounts:read"},
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
