package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestTenantScopeValidation(t *testing.T) {
	valid := TenantScope{TenantID: "00000000-0000-4000-8000-000000000001", ActorID: "operator-1"}
	for _, test := range []struct {
		name  string
		scope TenantScope
	}{
		{"missing tenant", TenantScope{ActorID: valid.ActorID}},
		{"nil tenant", TenantScope{TenantID: "00000000-0000-0000-0000-000000000000", ActorID: valid.ActorID}},
		{"malformed tenant", TenantScope{TenantID: "tenant-one", ActorID: valid.ActorID}},
		{"missing actor", TenantScope{TenantID: valid.TenantID}},
		{"padded actor", TenantScope{TenantID: valid.TenantID, ActorID: " operator"}},
		{"control actor", TenantScope{TenantID: valid.TenantID, ActorID: "operator\nadmin"}},
		{"long actor", TenantScope{TenantID: valid.TenantID, ActorID: strings.Repeat("x", 257)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			err := WithTenantTransaction(context.Background(), nil, test.scope, nil, func(*sql.Tx) error { called = true; return nil })
			if !errors.Is(err, ErrInvalidTenantScope) || called {
				t.Fatalf("invalid scope reached callback or wrong error: %v", err)
			}
		})
	}
}
