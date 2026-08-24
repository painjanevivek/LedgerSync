package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCProvider verifies Cognito-style OAuth access tokens. Tenant authority is
// derived from a server-owned client mapping, never from a caller-controlled
// custom claim. The resource audience binds tokens to this API.
type OIDCProvider struct {
	verifier         *oidc.IDTokenVerifier
	resourceAudience string
	clientTenants    map[string]string
}

type OIDCProviderConfig struct {
	IssuerURL        string
	ResourceAudience string
	ClientTenants    map[string]string
}

type accessTokenClaims struct {
	ClientID string   `json:"client_id"`
	TokenUse string   `json:"token_use"`
	Scope    string   `json:"scope"`
	Audience []string `json:"-"`
}

func NewOIDCProvider(ctx context.Context, config OIDCProviderConfig) (*OIDCProvider, error) {
	config.IssuerURL = strings.TrimSpace(config.IssuerURL)
	config.ResourceAudience = strings.TrimSpace(config.ResourceAudience)
	if config.IssuerURL == "" || config.ResourceAudience == "" || len(config.ClientTenants) == 0 {
		return nil, errors.New("OIDC issuer URL, resource audience, and client tenant mapping are required")
	}
	discoveryContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(discoveryContext, config.IssuerURL)
	if err != nil {
		return nil, errors.New("discover OIDC provider")
	}
	clientTenants := make(map[string]string, len(config.ClientTenants))
	for clientID, tenantID := range config.ClientTenants {
		clientID, tenantID = strings.TrimSpace(clientID), strings.TrimSpace(tenantID)
		if clientID == "" || tenantID == "" {
			return nil, errors.New("OIDC client tenant mapping is invalid")
		}
		clientTenants[clientID] = tenantID
	}
	return &OIDCProvider{verifier: provider.Verifier(&oidc.Config{
		SkipClientIDCheck:    true,
		SupportedSigningAlgs: []string{"RS256", "ES256", "PS256"},
	}), resourceAudience: config.ResourceAudience, clientTenants: clientTenants}, nil
}

func (p *OIDCProvider) Authenticate(ctx context.Context, credential string) (Principal, error) {
	if p == nil || p.verifier == nil || strings.TrimSpace(credential) == "" {
		return Principal{}, ErrUnauthenticated
	}
	token, err := p.verifier.Verify(ctx, credential)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	var claims accessTokenClaims
	if err := token.Claims(&claims); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	claims.Audience = append([]string(nil), token.Audience...)
	return principalFromAccessTokenClaims(claims, p.resourceAudience, p.clientTenants)
}

func principalFromAccessTokenClaims(claims accessTokenClaims, resourceAudience string, clientTenants map[string]string) (Principal, error) {
	claims.ClientID = strings.TrimSpace(claims.ClientID)
	if claims.TokenUse != "access" || claims.ClientID == "" || !containsAudience(claims.Audience, resourceAudience) {
		return Principal{}, ErrUnauthenticated
	}
	tenantID := strings.TrimSpace(clientTenants[claims.ClientID])
	if tenantID == "" {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{
		SubjectID: "oauth-client:" + claims.ClientID,
		TenantID:  tenantID,
		Roles:     map[string]struct{}{},
		Scopes:    allowedSet(strings.Fields(claims.Scope), allowedScopes),
	}, nil
}

func containsAudience(audiences []string, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return false
	}
	for _, audience := range audiences {
		if audience == required {
			return true
		}
	}
	return false
}

var allowedRoles = map[string]struct{}{
	"tenant:operator": {},
	"tenant:admin":    {},
}

var allowedScopes = map[string]struct{}{
	"accounts:read":        {},
	"accounts:write":       {},
	"transfers:read":       {},
	"transfers:write":      {},
	"transactions:read":    {},
	"reconciliation:read":  {},
	"reconciliation:write": {},
	"local:read":           {},
	"events:read":          {},
	"developer:read":       {},
	"audit:read":           {},
	BFFActorScope:          {},
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
