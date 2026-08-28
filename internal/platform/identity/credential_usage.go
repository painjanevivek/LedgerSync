package identity

import (
	"context"
	"errors"
	"strings"
	"time"
)

type CredentialUsageRecorder interface {
	RecordUsage(context.Context, string, string, time.Time) error
}

// UsageTrackingProvider records successful OAuth client use without receiving
// or persisting the bearer token. BFF actor assertions keep their human actor,
// while the authenticated workload client reference receives last-used evidence.
type UsageTrackingProvider struct {
	provider Provider
	recorder CredentialUsageRecorder
	clock    func() time.Time
}

func NewUsageTrackingProvider(provider Provider, recorder CredentialUsageRecorder, clock func() time.Time) (*UsageTrackingProvider, error) {
	if provider == nil || recorder == nil {
		return nil, errors.New("identity provider and credential usage recorder are required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &UsageTrackingProvider{provider: provider, recorder: recorder, clock: clock}, nil
}

func (p *UsageTrackingProvider) Authenticate(ctx context.Context, credential string) (Principal, error) {
	principal, err := p.provider.Authenticate(ctx, credential)
	if err != nil {
		return Principal{}, err
	}
	reference, tracked := strings.CutPrefix(principal.SubjectID, "oauth-client:")
	if !tracked || strings.TrimSpace(reference) == "" {
		return principal, nil
	}
	usageContext, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if err := p.recorder.RecordUsage(usageContext, principal.TenantID, reference, p.clock().UTC()); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return principal, nil
}
