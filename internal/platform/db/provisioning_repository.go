package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/provisioning"
)

type ProvisioningRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewProvisioningRepository(database *sql.DB, clock func() time.Time) (*ProvisioningRepository, error) {
	if database == nil {
		return nil, errors.New("provisioning database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &ProvisioningRepository{database: database, clock: clock}, nil
}

func (r *ProvisioningRepository) Apply(ctx context.Context, configuration provisioning.Config, pilotCurrency, actor, correlation string) error {
	if _, err := configuration.Validate(pilotCurrency); err != nil {
		return err
	}
	if actor == "" || correlation == "" {
		return errors.New("trusted actor and correlation ID are required")
	}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		return err
	}
	return WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: configuration.TenantID, ActorID: actor}, "provisioning|"+configuration.TenantID, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT public.controlled_apply_partner_v1($1,$2,$3,$4,$5,$6)`, configuration.TenantID, actor, correlation, string(encoded), pilotCurrency, r.clock().UTC())
		return err
	})
}

func (r *ProvisioningRepository) Rollback(ctx context.Context, tenantID, actor, correlation string) error {
	if tenantID == "" || actor == "" || correlation == "" {
		return errors.New("tenant, trusted actor, and correlation ID are required")
	}
	return WithTenantSerializableSequence(ctx, r.database, TenantScope{TenantID: tenantID, ActorID: actor}, "provisioning|"+tenantID, 5, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT public.controlled_rollback_partner_v1($1,$2,$3,$4)`, tenantID, actor, correlation, r.clock().UTC())
		return err
	})
}
