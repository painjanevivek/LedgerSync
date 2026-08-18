package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PoolConfig struct {
	DriverName      string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	PingTimeout     time.Duration
}

// OpenPool configures and verifies a database/sql pool. The executable selects
// its registered PostgreSQL driver; this package stays testable with sqlmock.
func OpenPool(ctx context.Context, cfg PoolConfig) (*sql.DB, error) {
	if strings.TrimSpace(cfg.DriverName) == "" || strings.TrimSpace(cfg.DSN) == "" {
		return nil, errors.New("database driver and DSN are required")
	}
	database, err := sql.Open(cfg.DriverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(max(cfg.MaxOpenConns, 4))
	database.SetMaxIdleConns(max(cfg.MaxIdleConns, 2))
	database.SetConnMaxLifetime(nonZero(cfg.ConnMaxLifetime, 30*time.Minute))
	pingCtx, cancel := context.WithTimeout(ctx, nonZero(cfg.PingTimeout, 3*time.Second))
	defer cancel()
	if err := database.PingContext(pingCtx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return database, nil
}

// WithSerializableRetry runs a complete financial transaction under PostgreSQL
// serializable isolation. Only proven transient conflicts are retried; callers
// must make every attempted operation idempotent inside the transaction.
func WithSerializableRetry(ctx context.Context, database *sql.DB, attempts int, fn func(*sql.Tx) error) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		tx, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return fmt.Errorf("begin serializable transaction: %w", err)
		}
		err = fn(tx)
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsRetryableTransactionError(err) || attempt == attempts-1 {
			break
		}
		wait := time.Duration(1<<attempt) * 25 * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

// IsRetryableTransactionError recognizes PostgreSQL serialization and deadlock
// SQLSTATEs without treating validation/authentication failures as retryable.
func IsRetryableTransactionError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "sqlstate 40001") || strings.Contains(text, "sqlstate 40p01") ||
		strings.Contains(text, "could not serialize access") || strings.Contains(text, "deadlock detected")
}

func nonZero(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
