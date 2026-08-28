package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

type usageProvider struct{ principal Principal }

func (p usageProvider) Authenticate(context.Context, string) (Principal, error) {
	return p.principal, nil
}

type usageRecorder struct {
	tenant, reference string
	at                time.Time
	err               error
}

func (r *usageRecorder) RecordUsage(_ context.Context, tenant, reference string, at time.Time) error {
	r.tenant, r.reference, r.at = tenant, reference, at
	return r.err
}

func TestUsageTrackingRecordsOnlyOAuthClientReference(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	recorder := &usageRecorder{}
	provider, err := NewUsageTrackingProvider(usageProvider{principal: Principal{SubjectID: "oauth-client:partner-001", TenantID: "tenant-1"}}, recorder, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	principal, err := provider.Authenticate(context.Background(), "opaque-token")
	if err != nil || principal.SubjectID != "oauth-client:partner-001" {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}
	if recorder.tenant != "tenant-1" || recorder.reference != "partner-001" || !recorder.at.Equal(now) {
		t.Fatalf("usage=%#v", recorder)
	}
}

func TestUsageTrackingFailsClosedWhenEvidenceCannotAdvance(t *testing.T) {
	recorder := &usageRecorder{err: errors.New("database unavailable")}
	provider, _ := NewUsageTrackingProvider(usageProvider{principal: Principal{SubjectID: "oauth-client:partner-001", TenantID: "tenant-1"}}, recorder, time.Now)
	if _, err := provider.Authenticate(context.Background(), "opaque-token"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error=%v", err)
	}
}
