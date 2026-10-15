package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// beginClaimTransaction resolves one live fenced claim through a narrow function.
// Every item query must still check owner, generation and expiry: the lease can
// expire between resolution and processing. No caller can request a table name.
func beginClaimTransaction(ctx context.Context, database *sql.DB, queue, id, owner string, generation int64, options *sql.TxOptions) (*tenantTransaction, error) {
	var tenant sql.NullString
	err := database.QueryRowContext(ctx, `SELECT public.controlled_claim_tenant_v1($1,$2,$3,$4)`, queue, id, owner, generation).Scan(&tenant)
	if err != nil {
		return nil, err
	}
	if !tenant.Valid {
		return nil, ErrStaleClaim
	}
	return beginTenantTransaction(ctx, database, TenantScope{TenantID: tenant.String, ActorID: owner}, options)
}

func renewWorkerClaim(ctx context.Context, database *sql.DB, queue, owner, id string, generation int64, duration time.Duration) error {
	if duration <= 0 || duration > 10*time.Minute {
		return errors.New("bounded lease duration is required")
	}
	tx, err := beginClaimTransaction(ctx, database, queue, id, owner, generation, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var renewed bool
	if err := tx.QueryRowContext(ctx, `SELECT public.controlled_renew_claim_v1($1,$2,$3,$4,$5)`, queue, id, owner, generation, duration.Seconds()).Scan(&renewed); err != nil {
		return err
	}
	if !renewed {
		return ErrStaleClaim
	}
	return tx.Commit()
}
