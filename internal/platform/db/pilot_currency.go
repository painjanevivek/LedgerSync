package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ValidatePilotCurrency refuses startup when any provisioned tenant policy,
// account, or transfer is outside the selected one-currency pilot boundary.
func ValidatePilotCurrency(ctx context.Context, database *sql.DB, currency string) error {
	var valid bool
	err := database.QueryRowContext(ctx, `SELECT public.controlled_pilot_currency_valid_v1($1)`, currency).Scan(&valid)
	if err != nil {
		return fmt.Errorf("validate pilot currency: %w", err)
	}
	if !valid {
		return fmt.Errorf("pilot currency %s invariant violated", currency)
	}
	return nil
}
