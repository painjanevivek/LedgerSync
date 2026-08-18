package integration_test

import (
	"context"
	"testing"
)

func TestMigrationsAreForwardCompatibleAndRepeatable(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	var versions int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 5 {
		t.Fatalf("migration versions=%d, want 5", versions)
	}
	for _, table := range []string{"accounts", "ledger_postings", "outbox_events", "reconciliation_runs"} {
		var exists bool
		if err := database.QueryRowContext(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("required table %s is missing after migration", table)
		}
	}
}
