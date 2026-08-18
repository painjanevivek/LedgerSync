package fault_test

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
	"github.com/redis/go-redis/v9"
)

const (
	faultTenantID      = "00000000-0000-0000-0000-000000000101"
	faultSourceID      = "00000000-0000-0000-0000-000000000110"
	faultDestinationID = "00000000-0000-0000-0000-000000000120"
	faultActorID       = "fault-operator"
)

func requireFaultDependencies(t *testing.T, sourceBalance int64) (*transfers.Service, *sql.DB, redis.UniversalClient) {
	t.Helper()
	databaseURL, redisAddress := os.Getenv("LEDGERSYNC_TEST_DATABASE_URL"), os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")
	if databaseURL == "" || redisAddress == "" {
		t.Skip("LEDGERSYNC_TEST_DATABASE_URL and LEDGERSYNC_TEST_REDIS_ADDR are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: databaseURL})
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
	if _, err := database.ExecContext(ctx, `TRUNCATE TABLE audit_events, outbox_events, idempotency_requests, ledger_postings, journal_transactions, transfers, account_balance_projections, account_owners, accounts, tenants CASCADE`); err != nil {
		t.Fatalf("clear test database: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants (id, external_reference) VALUES ($1, 'fault-tenant')`, faultTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, currency, status) VALUES ($1, $3, 'USD', 'active'), ($2, $3, 'USD', 'active')`, faultSourceID, faultDestinationID, faultTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_owners (tenant_id, account_id, subject_id, permission) VALUES ($1, $2, $3, 'debit')`, faultTenantID, faultSourceID, faultActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_balance_projections (account_id, available_minor, ledger_minor, balance_version) VALUES ($1, $3, $3, 0), ($2, 2000, 2000, 0)`, faultSourceID, faultDestinationID, sourceBalance); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_opening_balances (account_id, opening_ledger_minor) VALUES ($1, $3), ($2, 2000)`, faultSourceID, faultDestinationID, sourceBalance); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewTransferRepository(database, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := transfers.NewService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddress, DB: 0})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect test redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("clear test redis: %v", err)
	}
	return service, database, client
}

func faultTransferCommand(t *testing.T, key, amount string) transfers.Command {
	t.Helper()
	value, err := money.Parse("USD", amount)
	if err != nil {
		t.Fatal(err)
	}
	return transfers.Command{TenantID: faultTenantID, ActorSubjectID: faultActorID, DebitAccountID: faultSourceID, CreditAccountID: faultDestinationID, Amount: value, IdempotencyKey: key, CorrelationID: "00000000-0000-0000-0000-000000000199"}
}
