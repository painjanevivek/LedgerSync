package db

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
)

func classifyRecoveryCommandError(err error, delivery bool) error {
	if err == nil {
		return nil
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return err
	}
	switch postgresError.ConstraintName {
	case "controlled_recovery_outbox_not_found":
		return ErrDeadOutboxNotFound
	case "controlled_recovery_delivery_not_found":
		return ErrDeadDeliveryNotFound
	case "controlled_recovery_not_approved":
		if delivery {
			return ErrDeliveryReplayNotApproved
		}
		return ErrReplayNotApproved
	case "controlled_recovery_separation":
		return ErrReplaySeparationRequired
	case "controlled_recovery_idempotency":
		if delivery {
			return ErrDeliveryReplayIdempotencyConflict
		}
		return errors.New("replay approval correlation is already bound to different input")
	}
	return err
}
