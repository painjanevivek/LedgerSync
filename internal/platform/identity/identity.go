// Package identity exposes principals rather than protocol-specific identity
// details. HTTP handlers and use cases never trust browser-supplied user IDs.
package identity

import (
	"context"
	"errors"
	"time"
)

var ErrUnauthenticated = errors.New("unauthenticated")
var ErrUnauthorized = errors.New("unauthorized")

type Principal struct {
	SubjectID string
	TenantID  string
	Roles     map[string]struct{}
	Scopes    map[string]struct{}
	// AuthenticatedAt is established only by a verified identity token or a
	// signed BFF actor assertion. Sensitive commands use it as step-up evidence.
	AuthenticatedAt time.Time
}

func (p Principal) HasScope(scope string) bool {
	_, ok := p.Scopes[scope]
	return ok
}

func RequireScope(principal Principal, scope string) error {
	if !principal.HasScope(scope) {
		return ErrUnauthorized
	}
	return nil
}

func (p Principal) HasRole(role string) bool {
	_, ok := p.Roles[role]
	return ok
}

type Provider interface {
	Authenticate(context.Context, string) (Principal, error)
}
