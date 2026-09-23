package db

import (
	"context"
	"errors"
	"testing"
)

func TestSetLocalTenantContextRejectsNonCanonicalIdentifiersBeforeSQL(t *testing.T) {
	for _, tenantID := range []string{"", "not-a-uuid", " 00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000001 "} {
		t.Run(tenantID, func(t *testing.T) {
			if err := SetLocalTenantContext(context.Background(), nil, tenantID); !errors.Is(err, ErrInvalidTenantContext) {
				t.Fatalf("SetLocalTenantContext(%q) error=%v, want ErrInvalidTenantContext", tenantID, err)
			}
		})
	}
}
