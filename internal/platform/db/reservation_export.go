package db

import (
	"context"
	"strconv"

	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
)

func (r *ReservationRepository) Snapshot(ctx context.Context, scope hold.Scope) (result hold.Snapshot, err error) {
	tx, err := beginTenantRead(ctx, r.database, TenantScope{scope.TenantID, scope.ActorSubjectID})
	if err != nil {
		return result, reservationError(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = tx.QueryRowContext(ctx, `SELECT transaction_timestamp()`).Scan(&result.AsOf); err != nil {
		return result, reservationError(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+holdReadColumns+` FROM account_holds h WHERE h.tenant_id=$1 AND `+holdReadVisibility+` ORDER BY h.created_at,h.id LIMIT `+strconv.Itoa(hold.MaxExportHolds+1), scope.TenantID, scope.ActorSubjectID)
	if err != nil {
		return result, reservationError(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		item, e := scanReservation(rows)
		if e != nil {
			return result, e
		}
		result.Holds = append(result.Holds, item)
		if len(result.Holds) > hold.MaxExportHolds {
			return result, hold.ErrLimit
		}
	}
	return result, reservationError(rows.Err())
}
