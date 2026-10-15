package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/delivery"
)

type DeliveryRepository struct{ database *sql.DB }

func NewDeliveryRepository(database *sql.DB) (*DeliveryRepository, error) {
	if database == nil {
		return nil, errors.New("delivery database is required")
	}
	return &DeliveryRepository{database: database}, nil
}

// Record imports one historical attempt through an administrative capability.
// No runtime workload receives this capability. Workers use fenced job completion.
func (r *DeliveryRepository) Record(ctx context.Context, attempt delivery.Attempt) error {
	if err := delivery.Validate(attempt); err != nil {
		return err
	}
	err := WithTenantTransaction(ctx, r.database, TenantScope{TenantID: attempt.TenantID, ActorID: "system:delivery"}, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT public.controlled_import_delivery_evidence_v1($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, attempt.ID, attempt.TenantID, attempt.TransferID, nullableUUID(attempt.OutboxEventID), attempt.Kind, attempt.EndpointReference, attempt.AttemptNumber, attempt.Status, nullableString(attempt.ResponseClass), nullableString(attempt.SanitizedErrorCode), attempt.DueAt, nullableTime(attempt.StartedAt), nullableTime(attempt.CompletedAt))
		return err
	})
	if err != nil {
		return fmt.Errorf("record delivery attempt: %w", err)
	}
	return nil
}
