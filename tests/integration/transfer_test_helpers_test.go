package integration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

const (
	testTenantID      = "00000000-0000-0000-0000-000000000001"
	testSourceID      = "00000000-0000-0000-0000-000000000010"
	testDestinationID = "00000000-0000-0000-0000-000000000020"
	testActorID       = "integration-operator"
)

func requireTransferService(t *testing.T, sourceBalance int64) (*transfers.Service, *sql.DB) {
	t.Helper()
	if os.Getenv("LEDGERSYNC_TEST_DATABASE_URL") == "" {
		t.Skip("LEDGERSYNC_TEST_DATABASE_URL is required for PostgreSQL transfer integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration directory")
	}
	if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: os.DirFS(filepath.Join(filepath.Dir(sourceFile), "..", "..", "migrations"))}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := seedTransferFixture(ctx, database, sourceBalance); err != nil {
		t.Fatalf("seed transfer fixture: %v", err)
	}
	repository, err := db.NewTransferRepository(database, func() time.Time { return time.Date(2026, 8, 18, 9, 15, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("create transfer repository: %v", err)
	}
	service, err := transfers.NewService(repository, func() time.Time { return time.Date(2026, 8, 18, 9, 15, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("create transfer service: %v", err)
	}
	return service, database
}

func transferCommand(t *testing.T, key, amount string) transfers.Command {
	t.Helper()
	m, err := money.Parse("USD", amount)
	if err != nil {
		t.Fatal(err)
	}
	return transfers.Command{
		TenantID: testTenantID, ActorSubjectID: testActorID, DebitAccountID: testSourceID, CreditAccountID: testDestinationID,
		Amount: m, IdempotencyKey: key, CorrelationID: "00000000-0000-0000-0000-000000000099",
	}
}

func seedTransferFixture(ctx context.Context, database *sql.DB, sourceBalance int64) error {
	if _, err := database.ExecContext(ctx, `
TRUNCATE TABLE audit_events, outbox_events, idempotency_requests, ledger_postings, journal_transactions,
    transfers, account_balance_projections, account_owners, accounts, tenants CASCADE`); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants (id, external_reference) VALUES ($1, 'integration-tenant')`, testTenantID); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO accounts (id, tenant_id, currency, status) VALUES
    ($1, $3, 'USD', 'active'), ($2, $3, 'USD', 'active')`, testSourceID, testDestinationID, testTenantID); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_owners (tenant_id, account_id, subject_id, permission) VALUES ($1, $2, $3, 'debit')`, testTenantID, testSourceID, testActorID); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_credit_permissions (tenant_id,account_id,subject_id) VALUES ($1,$2,$3)`, testTenantID, testDestinationID, testActorID); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_transfer_policies (tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor) VALUES ($1,'USD',1,1000000000,5000000000,5000000000,10000000000)`, testTenantID); err != nil {
		return err
	}
	_, err := database.ExecContext(ctx, `
INSERT INTO account_balance_projections (account_id, available_minor, ledger_minor, balance_version) VALUES
    ($1, $3, $3, 0), ($2, 2000, 2000, 0)`, testSourceID, testDestinationID, sourceBalance)
	if err != nil {
		return err
	}
	_, err = database.ExecContext(ctx, `
INSERT INTO account_opening_balances (account_id, opening_ledger_minor) VALUES
    ($1, $3), ($2, 2000)`, testSourceID, testDestinationID, sourceBalance)
	return err
}

func countRows(t *testing.T, database *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
