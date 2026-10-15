package integration_test

import (
	"context"
	"database/sql"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

// Direct function tests still use the same authenticated transaction boundary
// as production. Scan completes before commit, so no row escapes its scope.
type tenantSQLRow struct {
	ctx                  context.Context
	database             *sql.DB
	tenant, actor, query string
	args                 []any
}

func tenantTestRow(ctx context.Context, database *sql.DB, tenant, actor, query string, args ...any) tenantSQLRow {
	return tenantSQLRow{ctx, database, tenant, actor, query, args}
}
func (row tenantSQLRow) Scan(dest ...any) error {
	return db.WithTenantTransaction(row.ctx, row.database, db.TenantScope{TenantID: row.tenant, ActorID: row.actor}, nil, func(tx *sql.Tx) error {
		return tx.QueryRowContext(row.ctx, row.query, row.args...).Scan(dest...)
	})
}
func tenantTestExec(ctx context.Context, database *sql.DB, tenant, actor, query string, args ...any) (sql.Result, error) {
	var result sql.Result
	err := db.WithTenantTransaction(ctx, database, db.TenantScope{TenantID: tenant, ActorID: actor}, nil, func(tx *sql.Tx) error {
		var err error
		result, err = tx.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}
