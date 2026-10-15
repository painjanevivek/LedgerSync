package integration_test

import (
	"context"
	"sort"
	"testing"
	"testing/fstest"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// The host profile binds the administrator URL before these tests create their
// own databases. No historical database is reset to obtain an upgrade pass.
func TestPortfolioRecoveryMigrations(t *testing.T) {
	recoveryControl(t)
	t.Run("oldest-supported-local-snapshot", TestMigrationThirteenUpgradesPhaseSevenDataWithoutFinancialRewrite)
	t.Run("known-legacy-room-history", TestLiveRoomMigrationPreservesKnownLegacyRecordsAndHistory)
	t.Run("previous-candidate", testPreviousCandidateUpgrade)
	t.Run("interrupted-migration-resumes", testInterruptedMigrationResumes)
	t.Run("bounded-retention", TestRetentionIsBoundedAndProtectsFinalEvidence)
}

func testPreviousCandidateUpgrade(t *testing.T) {
	database := liveMigrationDatabase(t)
	ctx := context.Background()
	source := liveMigrationSource(t, false)
	names := make([]string, 0, len(source))
	for name := range source {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) < 2 {
		t.Fatal("previous candidate requires ordered migration history")
	}
	latest := names[len(names)-1]
	previous := fstest.MapFS{}
	for _, name := range names[:len(names)-1] {
		previous[name] = source[name]
	}
	if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: previous}); err != nil {
		t.Fatal(err)
	}
	if err := seedTransferFixture(ctx, database, 10000); err != nil {
		t.Fatal(err)
	}
	var before, after string
	const financial = `SELECT jsonb_build_object('accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),'balances',(SELECT jsonb_agg(to_jsonb(b) ORDER BY account_id) FROM account_balance_projections b),'owners',(SELECT jsonb_agg(to_jsonb(o) ORDER BY account_id,subject_id) FROM account_owners o))::text`
	if err := database.QueryRowContext(ctx, financial).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: source}); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, financial).Scan(&after); err != nil || before != after {
		t.Fatal("previous candidate financial records changed", err)
	}
	if countRows(t, database, `SELECT count(*) FROM schema_migrations`) != len(names) || countRows(t, database, `SELECT count(*) FROM schema_migrations WHERE version=$1`, latest) != 1 {
		t.Fatal("upgrade history is incomplete")
	}
	// A second setup attempt must preserve the same history and records.
	if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: source}); err != nil {
		t.Fatal(err)
	}
	t.Logf("previous=%s candidate=%s history=%d financial_rows=unchanged", names[len(names)-2], latest, len(names))
}

func testInterruptedMigrationResumes(t *testing.T) {
	database := liveMigrationDatabase(t)
	ctx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	held, err := database.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := held.Close(); err != nil {
			t.Errorf("close migration test connection: %v", err)
		}
	}()
	const lock = int64(710202610)
	if _, err := held.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lock); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := held.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, lock); err != nil {
			t.Errorf("release migration test lock: %v", err)
		}
	}()
	source := fstest.MapFS{"000001_interrupt_probe.up.sql": {Data: []byte(`CREATE TABLE portfolio_interrupt_probe(id integer PRIMARY KEY); INSERT INTO portfolio_interrupt_probe VALUES (1); SELECT pg_advisory_xact_lock(710202610);`)}}
	interrupted, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- db.ApplyPending(interrupted, database, db.MigrationConfig{Source: source}) }()
	blocked := false
	for i := 0; i < 200; i++ {
		var waiting bool
		if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND query LIKE '%portfolio_interrupt_probe%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("migration did not reach its real blocked transaction")
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("interrupted migration reported success")
	}
	var exists bool
	if err := database.QueryRowContext(ctx, `SELECT to_regclass('public.portfolio_interrupt_probe') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal("interrupted schema transaction did not roll back", err)
	}
	if countRows(t, database, `SELECT count(*) FROM schema_migrations`) != 0 {
		t.Fatal("interrupted migration recorded completion")
	}
	if _, err := held.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, lock); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: source}); err != nil {
			t.Fatal(err)
		}
	}
	if countRows(t, database, `SELECT count(*) FROM portfolio_interrupt_probe`) != 1 || countRows(t, database, `SELECT count(*) FROM schema_migrations`) != 1 {
		t.Fatal("migration retry did not commit exactly once")
	}
}
