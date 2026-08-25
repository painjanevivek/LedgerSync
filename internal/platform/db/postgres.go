package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/rand/v2"
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
		pingErr := fmt.Errorf("ping database: %w", err)
		if closeErr := database.Close(); closeErr != nil {
			return nil, errors.Join(pingErr, fmt.Errorf("close failed database pool: %w", closeErr))
		}
		return nil, pingErr
	}
	return database, nil
}

type transactionBeginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func withSerializableRetry(ctx context.Context, database transactionBeginner, attempts int, fn func(*sql.Tx) error) error {
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
		// Jitter prevents concurrent transfers on the same accounts from
		// repeatedly colliding in lockstep. The bound keeps five attempts well
		// inside the BFF's two-second unknown-outcome timeout.
		base := time.Duration(1<<attempt) * 10 * time.Millisecond
		wait := base + time.Duration(rand.Int64N(int64(base)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

// WithSerializableSequence acquires a PostgreSQL session advisory lock before
// opening the serializable transaction. It is used only where one exact policy
// row necessarily serializes all writes in the named scope. Acquiring the lock
// before BeginTx gives every queued request a fresh serializable snapshot and
// prevents avoidable retry storms across multiple API instances.
func WithSerializableSequence(ctx context.Context, database *sql.DB, sequenceKey string, attempts int, fn func(*sql.Tx) error) (err error) {
	if strings.TrimSpace(sequenceKey) == "" {
		return errors.New("serializable sequence key is required")
	}
	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve serializable sequence connection: %w", err)
	}
	defer func() { _ = connection.Close() }()
	if _, err = connection.ExecContext(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, sequenceKey); err != nil {
		return fmt.Errorf("acquire serializable sequence: %w", err)
	}
	defer func() {
		unlockContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, unlockErr := connection.ExecContext(unlockContext, `SELECT pg_advisory_unlock_all()`); unlockErr != nil {
			// A connection with an uncertain session-lock state must never return
			// to the pool. driver.ErrBadConn instructs database/sql to discard it.
			_ = connection.Raw(func(any) error { return driver.ErrBadConn })
			if err == nil {
				err = fmt.Errorf("release serializable sequence: %w", unlockErr)
			}
		}
	}()
	return withSerializableRetry(ctx, connection, attempts, fn)
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
