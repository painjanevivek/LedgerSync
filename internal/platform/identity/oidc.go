package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCProvider verifies bearer ID tokens with the issuer's discovered key set.
// Tenant membership is a required, provider-mapped claim; roles and scopes are
// allowlisted so an unexpected custom claim never grants authority.
type OIDCProvider struct{ verifier *oidc.IDTokenVerifier }

func NewOIDCProvider(ctx context.Context, issuerURL, audience string) (*OIDCProvider, error) {
	issuerURL, audience = strings.TrimSpace(issuerURL), strings.TrimSpace(audience)
	if issuerURL == "" || audience == "" {
		return nil, errors.New("OIDC issuer URL and audience are required")
	}
	discoveryContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(discoveryContext, issuerURL)
	if err != nil {
		return nil, errors.New("discover OIDC provider")
	}
	return &OIDCProvider{verifier: provider.Verifier(&oidc.Config{
		ClientID:             audience,
		SupportedSigningAlgs: []string{"RS256", "ES256", "PS256"},
	})}, nil
}

func (p *OIDCProvider) Authenticate(ctx context.Context, credential string) (Principal, error) {
	if p == nil || p.verifier == nil || strings.TrimSpace(credential) == "" {
		return Principal{}, ErrUnauthenticated
	}
	token, err := p.verifier.Verify(ctx, credential)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	var claims struct {
		Subject  string   `json:"sub"`
		TenantID string   `json:"tenant_id"`
		Roles    []string `json:"roles"`
		Scope    string   `json:"scope"`
	}
	if err := token.Claims(&claims); err != nil || strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.TenantID) == "" {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{
		SubjectID: claims.Subject,
		TenantID:  claims.TenantID,
		Roles:     allowedSet(claims.Roles, allowedRoles),
		Scopes:    allowedSet(strings.Fields(claims.Scope), allowedScopes),
	}, nil
}

var allowedRoles = map[string]struct{}{
	"tenant:operator": {},
	"tenant:admin":    {},
}

var allowedScopes = map[string]struct{}{
	"accounts:read":     {},
	"transfers:write":   {},
	"transactions:read": {},
	"audit:read":        {},
	BFFActorScope:       {},
}

func allowedSet(values []string, allowed map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for _, value := range values {
		if _, ok := allowed[value]; ok {
			result[value] = struct{}{}
		}
	}
	return result
}
