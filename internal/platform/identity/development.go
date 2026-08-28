package identity

import (
	"context"
	"strings"
	"time"
)

// DevelopmentProvider exists only for explicit local development. Production
// wiring must use the OIDC provider; this adapter accepts no browser header.
type DevelopmentProvider struct {
	SubjectID  string
	TenantID   string
	Credential string
	Roles      []string
	Scopes     []string
}

func (p DevelopmentProvider) Authenticate(_ context.Context, credential string) (Principal, error) {
	expected := strings.TrimSpace(p.Credential)
	if expected == "" {
		expected = "development-local-only"
	}
	if strings.TrimSpace(p.SubjectID) == "" || credential != expected {
		return Principal{}, ErrUnauthenticated
	}
	roles := make(map[string]struct{}, len(p.Roles))
	for _, role := range p.Roles {
		roles[role] = struct{}{}
	}
	scopes := make(map[string]struct{}, len(p.Scopes))
	for _, scope := range p.Scopes {
		scopes[scope] = struct{}{}
	}
	return Principal{SubjectID: p.SubjectID, TenantID: p.TenantID, Roles: roles, Scopes: scopes, AuthenticatedAt: time.Now().UTC()}, nil
}
