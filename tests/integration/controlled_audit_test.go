package integration_test

import (
	"context"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"testing"
	"time"
)

func TestAuditAppendWithoutDirectInsert(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	if _, err := admin.Exec(`REVOKE INSERT ON audit_events FROM ledgersync_api`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provisionLiveMigrationRoles(t, admin) })
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repo, _ := db.NewAuditRepository(api.db)
	event := db.AuditEvent{TenantID: testTenantID, ActorSubjectID: testActorID, EventType: "transfer.policy_denied", TargetType: "transfer_request", Outcome: "failed", CorrelationID: "00000000-0000-4000-8000-000000000993", OccurredAt: time.Now(), Metadata: map[string]string{"operation": "transfer", "secret": "must-not-persist"}}
	if err := repo.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM audit_events WHERE correlation_id=$1 AND sanitized_metadata->>'operation'='transfer' AND NOT sanitized_metadata ? 'secret'`, event.CorrelationID); got != 1 {
		t.Fatalf("sanitized audit count=%d", got)
	}
	for _, metadata := range []string{`{"secret":"forbidden"}`, `{"operation":{"nested":"value"}}`, `{"operation":"line\nbreak"}`} {
		_, err := tenantTestExec(context.Background(), api.db, testTenantID, testActorID, `SELECT controlled_append_audit_v1(gen_random_uuid(),$1,$2,'transfer.policy_denied','transfer_request',NULL,'failed',$3,$4::jsonb,now())`, testTenantID, testActorID, event.CorrelationID, metadata)
		if sqlState(err) != "22023" {
			t.Fatalf("unsafe direct audit metadata accepted: %v", err)
		}
	}
	if got := countRows(t, admin, `SELECT count(*) FROM audit_events WHERE correlation_id=$1`, event.CorrelationID); got != 1 {
		t.Fatal("rejected audit metadata left evidence")
	}
}
