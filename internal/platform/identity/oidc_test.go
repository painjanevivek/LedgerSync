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
	payload, _ := json.Marshal(actorAssertionPayload{SubjectID: "customer-user", TenantID: "tenant-a", Scopes: []string{"accounts:read"}, ExpiresAt: now.Add(time.Minute).Unix()})
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
}
