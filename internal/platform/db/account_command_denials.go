package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
)

func persistAccountDenial(ctx context.Context, tx *sql.Tx, envelope commandEnvelope, operation, accountID string, commandErr error, committedDenial *error, extraMetadata ...map[string]string) error {
	code, safe := accountDenialCode(commandErr)
	if !safe {
		return commandErr
	}
	denial := accountDenialError(code)
	if denial == nil {
		return commandErr
	}
	metadata := map[string]string{"operation": operation, "denial_code": code}
	mergeAccountCommandMetadata(metadata, extraMetadata)
	if err := insertAccountAudit(ctx, tx, envelope, accountID, "account.command_denied", "denied", metadata); err != nil {
		return err
	}
	if err := storeAccountFailure(ctx, tx, envelope, code, accountDenialStatus(code)); err != nil {
		return err
	}
	*committedDenial = denial
	return nil
}

func (r *AccountCommandRepository) classifyUncommittedError(ctx context.Context, envelope commandEnvelope, operation, accountID string, err error, extraMetadata ...map[string]string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, idempotency.ErrConflict) || errors.Is(err, idempotency.ErrInProgress) {
		return err
	}
	code, safe := accountDenialCode(err)
	if safe {
		auditErr := func() error {
			tx, beginErr := r.database.BeginTx(ctx, nil)
			if beginErr != nil {
				return beginErr
			}
			defer func() { _ = tx.Rollback() }()
			var tenantExists bool
			if queryErr := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tenants WHERE id=$1)`, envelope.TenantID).Scan(&tenantExists); queryErr != nil || !tenantExists {
				return queryErr
			}
			metadata := map[string]string{"operation": operation, "denial_code": code}
			mergeAccountCommandMetadata(metadata, extraMetadata)
			if insertErr := insertAccountAudit(ctx, tx, envelope, accountID, "account.command_denied", "denied", metadata); insertErr != nil {
				return insertErr
			}
			return tx.Commit()
		}()
		if auditErr != nil {
			return fmt.Errorf("persist required account denial audit: %w", auditErr)
		}
		return err
	}
	return fmt.Errorf("%w: %v", accounts.ErrCommandUnavailable, err)
}

func mergeAccountCommandMetadata(destination map[string]string, sources []map[string]string) {
	if len(sources) == 0 {
		return
	}
	for key, value := range sanitizeAuditMetadata(sources[0]) {
		destination[key] = value
	}
}

func accountDenialCode(err error) (string, bool) {
	switch {
	case errors.Is(err, accounts.ErrAccountNotFound):
		return "not_found_or_not_authorized", true
	case errors.Is(err, accounts.ErrAccountConflict):
		return "reference_conflict", true
	case errors.Is(err, accountdomain.ErrInvalidTransition):
		return "invalid_transition", true
	case errors.Is(err, accountdomain.ErrTerminalStatus):
		return "terminal_status", true
	case errors.Is(err, accountdomain.ErrNonZeroBalance):
		return "non_zero_balance", true
	case errors.Is(err, accountdomain.ErrFinancialStateUnavailable):
		return "financial_state_unavailable", true
	case errors.Is(err, accountdomain.ErrVersionConflict):
		return "version_conflict", true
	default:
		return "", false
	}
}

func accountDenialError(code string) error {
	switch code {
	case "not_found_or_not_authorized":
		return accounts.ErrAccountNotFound
	case "reference_conflict":
		return accounts.ErrAccountConflict
	case "invalid_transition":
		return accounts.ErrInvalidTransition
	case "terminal_status":
		return accounts.ErrTerminalStatus
	case "non_zero_balance":
		return accounts.ErrNonZeroClose
	case "financial_state_unavailable":
		return accounts.ErrFinancialUnavailable
	case "version_conflict":
		return accounts.ErrVersionConflict
	default:
		return nil
	}
}

func accountDenialStatus(code string) int {
	switch code {
	case "not_found_or_not_authorized":
		return 404
	case "non_zero_balance", "financial_state_unavailable":
		return 422
	default:
		return 409
	}
}
