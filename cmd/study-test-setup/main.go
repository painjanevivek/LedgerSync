// Command study-test-setup provisions only the test harness database.
package main

import (
	"context"
	"fmt"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"net/url"
	"os"
	"time"
)

func main() {
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func setup() error {
	dsn := os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host != "127.0.0.1:35432" || parsed.Path != "/ledgersync_test" {
		return fmt.Errorf("isolated test database is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: dsn})
	if err != nil {
		return fmt.Errorf("test setup database unavailable")
	}
	defer func() { _ = database.Close() }()
	if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: os.DirFS("migrations")}); err != nil {
		return err
	}
	roles, err := os.ReadFile("deploy/postgres/roles.sql")
	if err != nil {
		return err
	}
	_, err = database.ExecContext(ctx, string(roles))
	return err
}
