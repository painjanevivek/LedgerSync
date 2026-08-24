// Command migrate is the only in-repository path that applies financial schema
// changes. Deploy it with a schema-privileged identity distinct from the API.
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, err := db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL})
	if err != nil {
		slog.Error("open migration database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()
	migrations := os.DirFS(filepath.Join(".", "migrations"))
	if err := db.ApplyPending(ctx, database, db.MigrationConfig{Source: migrations}); err != nil {
		slog.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")
}
