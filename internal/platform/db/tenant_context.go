package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

var ErrInvalidTenantScope = errors.New("valid tenant and actor scope required")

// TenantScope comes from an authenticated server boundary or an explicitly
// authorised maintenance command. It is never derived from diagnostic headers.
// Mutable PostgreSQL settings protect against missing query filters; they are
// not an identity boundary against a fully compromised shared workload.
type TenantScope struct {
	TenantID string
	ActorID  string
}

type scopedReadExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (scope TenantScope) canonical(ctx context.Context) (TenantScope, error) {
	tenant, err := identifier.Canonicalize(ctx, identifier.KindTenant, scope.TenantID)
	if err != nil || scope.ActorID == "" || len(scope.ActorID) > 256 || !utf8.ValidString(scope.ActorID) || strings.TrimSpace(scope.ActorID) != scope.ActorID || strings.ContainsFunc(scope.ActorID, unicode.IsControl) {
		return TenantScope{}, ErrInvalidTenantScope
	}
	scope.TenantID = tenant
	return scope, nil
}

func setTenantTransactionContext(ctx context.Context, tx *sql.Tx, scope TenantScope) error {
	// true is transaction-local. Commit, rollback, panic cleanup and cancellation
	// cannot leave this scope installed on a connection returned to the pool.
	_, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id',$1,true), set_config('app.actor_id',$2,true), set_config('app.diagnostic_traceparent',$3,true)`, scope.TenantID, scope.ActorID, observability.TraceParent(ctx))
	if err != nil {
		return fmt.Errorf("set transaction tenant context: %w", err)
	}
	return nil
}

// beginTenantRead gives a repository read a single scoped snapshot. The caller
// must defer Rollback even on success: this transaction can never persist writes.
func beginTenantRead(ctx context.Context, database *sql.DB, scope TenantScope) (*tenantTransaction, error) {
	return beginTenantTransaction(ctx, database, scope, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

// beginTenantTransaction also supports commands with multiple durable stages.
// Commit releases its reserved connection before a following stage begins.
// Every caller must defer Rollback to cover errors and panics.
func beginTenantTransaction(ctx context.Context, database *sql.DB, scope TenantScope, options *sql.TxOptions) (*tenantTransaction, error) {
	scope, err := scope.canonical(ctx)
	if err != nil {
		return nil, err
	}
	connection, err := reserveTenantConnection(ctx, database)
	if err != nil {
		return nil, err
	}
	tx, err := connection.BeginTx(ctx, options)
	if err != nil {
		releaseTenantConnection(connection)
		return nil, fmt.Errorf("begin tenant transaction: %w", err)
	}
	if err := setTenantTransactionContext(ctx, tx, scope); err != nil {
		_ = tx.Rollback()
		releaseTenantConnection(connection)
		return nil, err
	}
	return &tenantTransaction{Tx: tx, connection: connection}, nil
}

type tenantTransaction struct {
	*sql.Tx
	connection  *sql.Conn
	releaseOnce sync.Once
}

func (tx *tenantTransaction) Rollback() error {
	err := tx.Tx.Rollback()
	tx.releaseOnce.Do(func() { releaseTenantConnection(tx.connection) })
	return err
}

func (tx *tenantTransaction) Commit() error {
	err := tx.Tx.Commit()
	tx.releaseOnce.Do(func() { releaseTenantConnection(tx.connection) })
	return err
}

func resetTenantConnection(ctx context.Context, connection *sql.Conn) error {
	_, err := connection.ExecContext(ctx, `SELECT set_config('app.tenant_id','',false), set_config('app.actor_id','',false), set_config('app.diagnostic_traceparent','',false)`)
	return err
}

func reserveTenantConnection(ctx context.Context, database *sql.DB) (*sql.Conn, error) {
	if database == nil {
		return nil, errors.New("tenant transaction database required")
	}
	connection, err := database.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("reserve tenant connection: %w", err)
	}
	if err := resetTenantConnection(ctx, connection); err != nil {
		_ = connection.Raw(func(any) error { return driver.ErrBadConn })
		_ = connection.Close()
		return nil, fmt.Errorf("clear prior tenant scope: %w", err)
	}
	return connection, nil
}

func releaseTenantConnection(connection *sql.Conn) {
	// Use a bounded fresh context because the request can already be cancelled.
	// Discard any connection whose session state cannot be proved clear. Cleanup
	// failure must not turn an already committed financial result into a rejection.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := resetTenantConnection(ctx, connection); err != nil {
		_ = connection.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = connection.Close()
}

// WithTenantTransaction owns one transaction and its scope. The callback must
// use tx for every query and must close rows before returning. It must not
// commit, roll back, retain tx, or make external network calls.
func WithTenantTransaction(ctx context.Context, database *sql.DB, scope TenantScope, options *sql.TxOptions, fn func(*sql.Tx) error) error {
	scope, err := scope.canonical(ctx)
	if err != nil {
		return err
	}
	if database == nil || fn == nil {
		return errors.New("tenant transaction database and callback required")
	}
	tx, err := beginTenantTransaction(ctx, database, scope, options)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx.Tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	return nil
}

// WithTenantSerializableSequence preserves the existing command retry and lock
// boundary while installing context in each new serializable transaction.
func WithTenantSerializableSequence(ctx context.Context, database *sql.DB, scope TenantScope, sequenceKey string, attempts int, fn func(*sql.Tx) error) error {
	scope, err := scope.canonical(ctx)
	if err != nil {
		return err
	}
	if database == nil || fn == nil || attempts < 1 || attempts > 8 || strings.TrimSpace(sequenceKey) == "" {
		return errors.New("bounded tenant sequence and database required")
	}
	return withSerializableSequence(ctx, database, sequenceKey, attempts, func(tx *sql.Tx) error {
		defer func() {
			if value := recover(); value != nil {
				_ = tx.Rollback()
				panic(value)
			}
		}()
		if err := setTenantTransactionContext(ctx, tx, scope); err != nil {
			return err
		}
		return fn(tx)
	}, true)
}
