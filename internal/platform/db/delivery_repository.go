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

// Record appends one immutable attempt. A later retry is another row with the
// next attempt number; it never rewrites the financial transfer or old attempt.
func (r *DeliveryRepository) Record(ctx context.Context, attempt delivery.Attempt) error {
	if err := delivery.Validate(attempt); err != nil {
		return err
	}
	_, err := r.database.ExecContext(ctx, `INSERT INTO delivery_attempts (id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,response_class,sanitized_error_code,due_at,started_at,completed_at) VALUES ($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, attempt.ID, attempt.TenantID, attempt.TransferID, nullableUUID(attempt.OutboxEventID), attempt.Kind, attempt.EndpointReference, attempt.AttemptNumber, attempt.Status, nullableString(attempt.ResponseClass), nullableString(attempt.SanitizedErrorCode), attempt.DueAt, nullableTime(attempt.StartedAt), nullableTime(attempt.CompletedAt))
	if err != nil {
		return fmt.Errorf("record delivery attempt: %w", err)
	}
	return nil
}
