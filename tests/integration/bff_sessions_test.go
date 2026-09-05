package integration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// This test requires a disposable, migrated database; the normal local ledger
// is never selected implicitly. Exercise the actual PostgreSQL expressions and
// API role grants, which mocked session handlers cannot verify.
func TestOpaqueSessionsAndPreferencesOnPostgreSQL(t *testing.T) {
	harness := RequireHarness(t)
	ctx := context.Background()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: harness.DatabaseURL})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	tenant := uuid.NewString()
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants(id,external_reference) VALUES($1,$2)`, tenant, "session-smoke-"+tenant); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = database.ExecContext(ctx, `RESET ROLE`)
		_, _ = database.ExecContext(ctx, `DELETE FROM tenants WHERE id=$1`, tenant)
	}()
	if _, err := database.ExecContext(ctx, `SET ROLE ledgersync_api`); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewSessionRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	claims := db.SessionRecord{TenantID: tenant, SubjectID: "smoke-operator", CSRFToken: uuid.NewString(), Roles: []string{"tenant:operator"}, Scopes: []string{"accounts:read"}, ExpiresAt: time.Now().Add(10 * time.Minute), ConsistencyRequirements: map[string]string{}}
	token, err := repository.Create(ctx, claims, "")
	if err != nil {
		t.Fatalf("create opaque session: %v", err)
	}
	if len(token) != 43 {
		t.Fatal("unexpected opaque token length")
	}
	updates := map[string]string{}
	for i := 0; i < 10; i++ {
		updates[fmt.Sprintf("account-%d", i)] = "requirement"
	}
	if err := repository.UpdateConsistency(ctx, token, updates); err != nil {
		t.Fatalf("store ten consistency requirements: %v", err)
	}
	if err := repository.UpdateConsistency(ctx, token, map[string]string{"account-10": "overflow"}); err == nil {
		t.Fatal("eleventh requirement was accepted")
	}
	resolved, err := repository.Resolve(ctx, token)
	if err != nil || len(resolved.ConsistencyRequirements) != 10 {
		t.Fatalf("bounded requirements were not retained: %v", err)
	}
	rotated, err := repository.Create(ctx, claims, token)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := repository.Resolve(ctx, token); !errors.Is(err, db.ErrSessionNotFound) {
		t.Fatal("old token remains usable")
	}
	if err := repository.Revoke(ctx, rotated); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := repository.Resolve(ctx, rotated); !errors.Is(err, db.ErrSessionNotFound) {
		t.Fatal("revoked token remains usable")
	}
	preferences, err := db.NewUIPreferenceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := preferences.Get(ctx, tenant, claims.SubjectID)
	if err != nil || initial.ExperienceMode != "simple" || initial.Version != 0 {
		t.Fatal("missing preferences must default to simple")
	}
	updated, err := preferences.Update(ctx, tenant, claims.SubjectID, "expert", 0)
	if err != nil || updated.Version != 1 {
		t.Fatalf("persist preference: %v", err)
	}
	if _, err := preferences.Update(ctx, tenant, claims.SubjectID, "simple", 0); !errors.Is(err, db.ErrUIPreferenceConflict) {
		t.Fatal("stale preference version accepted")
	}
	other, err := preferences.Get(ctx, tenant, "different-operator")
	if err != nil || other.ExperienceMode != "simple" {
		t.Fatal("preference leaked across operators")
	}
}
