package integration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/guidance"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantGuidanceUsesOneScopedSnapshot(t *testing.T) {
	service, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	submission, err := service.Submit(ctx, transferCommand(t, "tenant-guidance-context-001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	// Use actual workload permissions. RLS is limited to this test fixture until
	// every production caller has a verified replacement.
	for _, table := range []string{"accounts", "transfers", "idempotency_requests", "journal_transactions", "outbox_events", "audit_events", "operator_onboarding_preferences"} {
		if _, err := admin.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ENABLE ROW LEVEL SECURITY;
CREATE POLICY portfolio_guidance_scope ON %s AS RESTRICTIVE USING (tenant_id=NULLIF(current_setting('app.tenant_id',true),'')::uuid AND NULLIF(current_setting('app.actor_id',true),'') IS NOT NULL)`, table, table)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := admin.ExecContext(context.Background(), fmt.Sprintf(`DROP POLICY portfolio_guidance_scope ON %s`, table)); err != nil {
				t.Error(err)
			}
		})
	}
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	session.db.SetMaxOpenConns(1)
	repository, err := db.NewGuidanceRepository(session.db)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("orientation retains authorized references", func(t *testing.T) {
		facts, err := repository.Orientation(ctx, testTenantID, testActorID)
		if err != nil || facts.AuthorizedAccount == nil || facts.PostedTransfer == nil {
			t.Fatalf("scoped orientation missing: %v", err)
		}
	})
	t.Run("preferences persist and keep actor boundaries", func(t *testing.T) {
		preference, err := repository.UpdateOrientationPreference(ctx, testTenantID, testActorID, guidance.PreferenceUpdate{Dismissed: true})
		if err != nil || preference.Version != 1 {
			t.Fatalf("scoped preference write: %v", err)
		}
		read, err := repository.OrientationPreference(ctx, testTenantID, testActorID)
		if err != nil || !read.Dismissed || read.Version != 1 {
			t.Fatalf("scoped preference read: %v", err)
		}
		other, err := repository.OrientationPreference(ctx, testTenantID, "different-actor")
		if err != nil || other.Dismissed || other.Version != 0 {
			t.Fatalf("actor preference leaked: %v", err)
		}
		if _, err := repository.UpdateOrientationPreference(ctx, testTenantID, testActorID, guidance.PreferenceUpdate{}); !errors.Is(err, guidance.ErrPreferenceConflict) {
			t.Fatalf("version conflict lost: %v", err)
		}
	})
	t.Run("timeline helpers share the scoped connection", func(t *testing.T) {
		facts, err := repository.ExplainTransfer(ctx, testTenantID, testActorID, submission.Result.TransferID)
		if err != nil {
			t.Fatal(err)
		}
		for name, link := range map[string]guidance.EvidenceLink{"request": facts.Request, "journal": facts.JournalPostings, "outbox": facts.Outbox, "versions": facts.BalanceVersions} {
			if link.Unavailable || len(link.Items) == 0 {
				t.Errorf("missing %s evidence in scoped snapshot", name)
			}
		}
		if _, err := repository.ExplainTransfer(ctx, testTenantID, "different-actor", submission.Result.TransferID); !errors.Is(err, guidance.ErrTransferNotFound) {
			t.Fatalf("actor boundary failed: %v", err)
		}
	})
	var count int
	if err := session.db.QueryRowContext(ctx, `SELECT count(*) FROM transfers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("guidance leaked session context: %v", err)
	}
}
