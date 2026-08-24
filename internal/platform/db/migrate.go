// Package db contains infrastructure that is explicitly invoked by commands.
// Financial schema changes are never run as a side effect of serving traffic.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

const migrationTable = "schema_migrations"

// MigrationConfig identifies the migration files controlled by a deployment.
// It is deliberately separate from API configuration so normal application
// startup cannot create or alter financial structures.
type MigrationConfig struct {
	Source fs.FS
}

// ApplyPending applies ordered *.up.sql files exactly once. It must be called
// by a dedicated deployment/migration command using a database account with
// schema privileges, never by the API or worker runtime identities.
func ApplyPending(ctx context.Context, database *sql.DB, cfg MigrationConfig) error {
	if database == nil {
		return fmt.Errorf("migration database is required")
	}
	if cfg.Source == nil {
		return fmt.Errorf("migration source is required")
	}

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS `+migrationTable+` (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}

	entries, err := fs.ReadDir(cfg.Source, ".")
	if err != nil {
		return fmt.Errorf("read migration source: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyOne(ctx, database, cfg.Source, name); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, database *sql.DB, source fs.FS, name string) error {
	var exists bool
	if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+migrationTable+` WHERE version = $1)`, name).Scan(&exists); err != nil {
		return fmt.Errorf("check migration %q: %w", name, err)
	}
	if exists {
		return nil
	}

	sqlBytes, err := fs.ReadFile(source, path.Clean(name))
	if err != nil {
		return fmt.Errorf("read migration %q: %w", name, err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %q: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+migrationTable+` (version, applied_at) VALUES ($1, $2)`, name, time.Now().UTC()); err != nil {
		return fmt.Errorf("record migration %q: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}
	return nil
}
