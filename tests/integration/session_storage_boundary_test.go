package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestSessionStorageRequiresExactDigestAndPreferencesRequireActor(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	workload := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	workload.db.SetMaxOpenConns(1)
	ctx := context.Background()
	repository, err := db.NewSessionRepository(workload.db)
	if err != nil {
		t.Fatal(err)
	}
	claims := db.SessionRecord{TenantID: testTenantID, SubjectID: "session-boundary-actor", CSRFToken: uuid.NewString(), Roles: []string{"tenant:operator"}, Scopes: []string{"accounts:read"}, ExpiresAt: time.Now().Add(10 * time.Minute), ConsistencyRequirements: map[string]string{}}
	token, err := repository.Create(ctx, claims, "")
	if err != nil {
		t.Fatal(err)
	}
	preferences, err := db.NewUIPreferenceRepository(workload.db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preferences.Update(ctx, testTenantID, claims.SubjectID, "expert", 0); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"bff_sessions", "operator_ui_preferences"} {
		var count int
		if err := workload.db.QueryRow("SELECT count(*) FROM public." + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("unscoped %s read count=%d err=%v", table, count, err)
		}
	}
	if err := db.WithTenantTransaction(ctx, workload.db, db.TenantScope{TenantID: testTenantID, ActorID: "different-operator"}, nil, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRow(`SELECT count(*) FROM operator_ui_preferences`).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Error("preferences leaked across operators")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	foreign := claims
	foreign.SubjectID = "different-operator"
	if _, err := repository.Create(ctx, foreign, token); !errors.Is(err, db.ErrSessionNotFound) {
		t.Fatalf("cross-principal rotation accepted: %v", err)
	}
	if record, err := repository.Resolve(ctx, token); err != nil || record.SubjectID != claims.SubjectID {
		t.Fatalf("failed rotation changed original session: %v", err)
	}
	if err := repository.UpdateConsistency(ctx, token, map[string]string{"account-boundary": "version-boundary"}); err != nil {
		t.Fatal(err)
	}
	var digest, rotation string
	if err := workload.db.QueryRow(`SELECT COALESCE(current_setting('app.bff_session_digest',true),''),COALESCE(current_setting('app.bff_rotation_digest',true),'')`).Scan(&digest, &rotation); err != nil || digest != "" || rotation != "" {
		t.Fatalf("session digest scope leaked into pool: %v", err)
	}
	if err := repository.Revoke(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Resolve(ctx, token); !errors.Is(err, db.ErrSessionNotFound) {
		t.Fatal("revoked session remained usable")
	}
}

func TestSessionExpiryCleanupIsBoundedAndCannotDeleteAnotherActor(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	ctx := context.Background()
	if _, err := admin.Exec(`INSERT INTO bff_sessions(token_digest,tenant_id,subject_id,csrf_token,created_at,expires_at)
 SELECT decode(md5('expired-session-'||n)||md5('expired-session-2-'||n),'hex'),$1,CASE WHEN n<=101 THEN 'expiry-actor' ELSE 'other-actor' END,'synthetic-csrf-token',now()-interval '3 hours',now()-CASE WHEN n<=101 THEN interval '1 hour' ELSE interval '2 hours' END FROM generate_series(1,102)n`, testTenantID); err != nil {
		t.Fatal(err)
	}
	workload := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, err := db.NewSessionRepository(workload.db)
	if err != nil {
		t.Fatal(err)
	}
	claims := db.SessionRecord{TenantID: testTenantID, SubjectID: "expiry-actor", CSRFToken: uuid.NewString(), Roles: []string{}, Scopes: []string{}, ExpiresAt: time.Now().Add(10 * time.Minute), ConsistencyRequirements: map[string]string{}}
	if _, err := repository.Create(ctx, claims, ""); err != nil {
		t.Fatal(err)
	}
	var own, other int
	if err := admin.QueryRow(`SELECT count(*) FILTER(WHERE subject_id='expiry-actor'),count(*) FILTER(WHERE subject_id='other-actor') FROM bff_sessions WHERE expires_at<now()`).Scan(&own, &other); err != nil || own != 1 || other != 1 {
		t.Fatalf("expiry cleanup own=%d other=%d err=%v", own, other, err)
	}
}
