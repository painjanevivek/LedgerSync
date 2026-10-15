package db

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// Session resolution precedes tenant discovery. The opaque digest is its exact
// row scope. Transaction-local settings and the normal pool reset prevent reuse.
func beginSessionTransaction(ctx context.Context, database *sql.DB, digest, rotation []byte, options *sql.TxOptions) (*tenantTransaction, error) {
	connection, err := reserveTenantConnection(ctx, database)
	if err != nil {
		return nil, err
	}
	tx, err := connection.BeginTx(ctx, options)
	if err != nil {
		releaseTenantConnection(connection)
		return nil, fmt.Errorf("begin session transaction: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.bff_session_digest',$1,true),set_config('app.bff_rotation_digest',$2,true)`, hex.EncodeToString(digest), hex.EncodeToString(rotation)); err != nil {
		_ = tx.Rollback()
		releaseTenantConnection(connection)
		return nil, fmt.Errorf("set session digest scope: %w", err)
	}
	return &tenantTransaction{Tx: tx, connection: connection}, nil
}
