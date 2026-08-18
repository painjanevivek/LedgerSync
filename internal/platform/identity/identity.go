// Package identity exposes principals rather than protocol-specific identity
// details. HTTP handlers and use cases never trust browser-supplied user IDs.
package identity

import (
	"context"
	"errors"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type Principal struct {
	SubjectID string
	TenantID  string
	Roles     map[string]struct{}
}

func (p Principal) HasRole(role string) bool {
	_, ok := p.Roles[role]
	return ok
}

type Provider interface {
	Authenticate(context.Context, string) (Principal, error)
}
