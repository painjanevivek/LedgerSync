package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"testing"
	"time"
)

func TestSchedulerLeaseTakeoverRejectsStaleJobs(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO reconciliation_scheduler_tenants(workload_name,tenant_id) VALUES(session_user,$1) ON CONFLICT DO NOTHING`, testTenantID); err != nil {
		t.Fatal(err)
	}
	var first, second int64
	if err := database.QueryRow(`SELECT acquire_reconciliation_scheduler($1,'node-a',500)`, testTenantID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	var blocked sql.NullInt64
	if err := database.QueryRow(`SELECT acquire_reconciliation_scheduler($1,'node-b',500)`, testTenantID).Scan(&blocked); err != nil || blocked.Valid {
		t.Fatalf("active lease replaced: %v %v", blocked, err)
	}
	var job string
	if err := database.QueryRow(`SELECT claim_reconciliation_schedule($1,'node-a',$2,15)`, testTenantID, first).Scan(&job); err != nil {
		t.Fatal(err)
	}
	var duplicate sql.NullString
	if err := database.QueryRow(`SELECT claim_reconciliation_schedule($1,'node-a',$2,15)`, testTenantID, first).Scan(&duplicate); err != nil || duplicate.Valid {
		t.Fatalf("duplicate schedule: %v %v", duplicate, err)
	}
	time.Sleep(550 * time.Millisecond)
	if err := database.QueryRow(`SELECT acquire_reconciliation_scheduler($1,'node-b',2000)`, testTenantID).Scan(&second); err != nil || second <= first {
		t.Fatalf("takeover fence: %d -> %d %v", first, second, err)
	}
	var renewed bool
	if err := database.QueryRow(`SELECT renew_reconciliation_scheduler($1,'node-a',$2,500)`, testTenantID, first).Scan(&renewed); err != nil || renewed {
		t.Fatalf("stale renewal accepted: %v %v", renewed, err)
	}
	if err := database.QueryRow(`SELECT claim_reconciliation_schedule($1,'node-a',$2,15)`, testTenantID, first).Scan(&duplicate); err == nil {
		t.Fatal("stale scheduler created work")
	}
	var recovered string
	if err := database.QueryRow(`SELECT claim_reconciliation_schedule($1,'node-b',$2,15)`, testTenantID, second).Scan(&recovered); err != nil || recovered != job {
		t.Fatalf("abandoned job not recovered: %s %s %v", job, recovered, err)
	}
	if _, err := database.ExecContext(ctx, `SELECT complete_reconciliation_schedule($1,'node-a',$2,$3,NULL,'matched')`, testTenantID, first, job); err == nil {
		t.Fatal("stale scheduler completed work")
	}
	repository, _ := db.NewReconciliationRepository(database)
	report, err := repository.Reconcile(ctx, testTenantID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `SELECT complete_reconciliation_schedule($1,'node-b',$2,$3,$4,'matched')`, testTenantID, second, job, report.ID); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT claim_reconciliation_schedule($1,'node-b',$2,15)`, testTenantID, second).Scan(&duplicate); err != nil {
		t.Fatal(err)
	}
	var completedSlot time.Time
	var completedState string
	var intervalCount int
	if err := database.QueryRow(`SELECT scheduled_for,state,(SELECT count(*) FROM reconciliation_scheduled_jobs same WHERE same.tenant_id=completed.tenant_id AND same.scheduled_for=completed.scheduled_for) FROM reconciliation_scheduled_jobs completed WHERE id=$1`, job).Scan(&completedSlot, &completedState, &intervalCount); err != nil {
		t.Fatal(err)
	}
	if completedState != "completed" || intervalCount != 1 {
		t.Fatalf("completed interval changed: state=%s count=%d", completedState, intervalCount)
	}
	if duplicate.Valid {
		// The database clock can cross a real 15-second boundary during this
		// takeover. A later interval is new work; the completed interval is not.
		var nextSlot time.Time
		if err := database.QueryRow(`SELECT scheduled_for FROM reconciliation_scheduled_jobs WHERE id=$1`, duplicate.String).Scan(&nextSlot); err != nil {
			t.Fatal(err)
		}
		if duplicate.String == job || !nextSlot.After(completedSlot) {
			t.Fatalf("completed interval repeated: %s", duplicate.String)
		}
	}
}

func TestSchedulerIdentityCannotReadOrWriteOtherTenants(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	role := "unit_scheduler_" + uuid.NewString()[:8]
	if _, err := database.Exec(fmt.Sprintf(`CREATE ROLE %s NOLOGIN INHERIT; GRANT ledgersync_scheduler TO %s`, role, role)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = database.Exec(fmt.Sprintf(`DROP ROLE %s`, role)) })
	if _, err := database.Exec(`INSERT INTO reconciliation_scheduler_tenants(workload_name,tenant_id) VALUES($1,$2)`, role, testTenantID); err != nil {
		t.Fatal(err)
	}
	const other = "00000000-0000-4000-8000-000000000888"
	if _, err := database.Exec(`INSERT INTO tenants(id,external_reference) VALUES($1,'scheduler-other-tenant')`, other); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close scheduler test connection: %v", err)
		}
	}()
	if _, err := conn.ExecContext(context.Background(), fmt.Sprintf(`SET SESSION AUTHORIZATION %s; SET search_path=ledgersync_scheduler,public`, role)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), `RESET SESSION AUTHORIZATION; RESET search_path`); err != nil {
			t.Errorf("reset scheduler test connection: %v", err)
		}
	}()
	var count int
	if err := conn.QueryRowContext(context.Background(), `SELECT count(*) FROM tenants`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("scoped tenant read: %d %v", count, err)
	}
	if _, err := conn.ExecContext(context.Background(), `SELECT * FROM public.ledger_postings`); err == nil {
		t.Fatal("scheduler read base financial tables")
	}
	if _, err := conn.ExecContext(context.Background(), `INSERT INTO public.transfers DEFAULT VALUES`); err == nil {
		t.Fatal("scheduler has transfer write authority")
	}
	if _, err := conn.ExecContext(context.Background(), `SELECT public.acquire_reconciliation_scheduler($1,'node',1000)`, other); err == nil {
		t.Fatal("scheduler acquired another tenant")
	}
}
