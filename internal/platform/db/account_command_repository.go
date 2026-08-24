package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
)

// AccountCommandRepository owns the PostgreSQL transaction boundary for
// account creation and lifecycle commands. It never updates a balance.
type AccountCommandRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewAccountCommandRepository(database *sql.DB, clock func() time.Time) (*AccountCommandRepository, error) {
	if database == nil {
		return nil, errors.New("account command database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &AccountCommandRepository{database: database, clock: clock}, nil
}

type commandEnvelope struct {
	TenantID, ActorID, CorrelationID, Key, Operation string
	OccurredAt                                       time.Time
}

func (r *AccountCommandRepository) Create(ctx context.Context, command accounts.CreateAccountCommand, fingerprint [sha256.Size]byte) (submission accounts.CommandSubmission, err error) {
	if command.OccurredAt.IsZero() {
		command.OccurredAt = r.clock().UTC()
	}
	envelope := commandEnvelope{command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey, accounts.CreateOperation, command.OccurredAt.UTC()}
	var committedDenial error
	err = WithSerializableSequence(ctx, r.database, "account-create|"+strings.ToLower(command.TenantID), 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, envelope.TenantID, envelope.ActorID); err != nil {
			return err
		}
		result, replay, replayedDenial, err := reserveAccountCommand(ctx, tx, envelope, fingerprint)
		if err != nil {
			return err
		}
		if replay {
			submission = accounts.CommandSubmission{Result: result, Replayed: true}
			committedDenial = replayedDenial
			return nil
		}
		accountID, err := newUUID()
		if err != nil {
			return err
		}
		aggregate, err := accountdomain.NewConfigured(accountID, command.TenantID, command.Currency, accountdomain.Metadata{DisplayName: command.DisplayName, ExternalReference: command.Reference, Category: command.Category}, accountdomain.Owner{SubjectID: command.ActorSubjectID, Permission: accountdomain.PermissionDebit}, envelope.OccurredAt)
		if err != nil {
			return err
		}
		if commandErr := insertAccountAggregate(ctx, tx, aggregate); commandErr != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_create", "", commandErr, &committedDenial)
		}
		result = accountCommandResult(aggregate, 0, 0)
		if err := insertAccountAudit(ctx, tx, envelope, aggregate.ID, "account.created", "succeeded", map[string]string{"status": string(aggregate.Status), "category": aggregate.Category}); err != nil {
			return err
		}
		if err := insertAccountOutbox(ctx, tx, envelope, aggregate, "account.created.v1"); err != nil {
			return err
		}
		if err := storeAccountOutcome(ctx, tx, envelope, result, 201); err != nil {
			return err
		}
		submission = accounts.CommandSubmission{Result: result}
		return nil
	})
	if err != nil {
		return submission, r.classifyUncommittedError(ctx, envelope, "account_create", "", err)
	}
	return submission, committedDenial
}

func (r *AccountCommandRepository) UpdateMetadata(ctx context.Context, command accounts.UpdateAccountMetadataCommand, fingerprint [sha256.Size]byte) (submission accounts.CommandSubmission, err error) {
	if command.OccurredAt.IsZero() {
		command.OccurredAt = r.clock().UTC()
	}
	envelope := commandEnvelope{command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey, accounts.UpdateOperation, command.OccurredAt.UTC()}
	var committedDenial error
	err = WithSerializableSequence(ctx, r.database, "account-update|"+strings.ToLower(command.TenantID)+"|"+strings.ToLower(command.AccountID), 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, envelope.TenantID, envelope.ActorID); err != nil {
			return err
		}
		result, replay, replayedDenial, err := reserveAccountCommand(ctx, tx, envelope, fingerprint)
		if err != nil {
			return err
		}
		if replay {
			submission = accounts.CommandSubmission{Result: result, Replayed: true}
			committedDenial = replayedDenial
			return nil
		}
		aggregate, available, ledger, err := lockOwnedAccount(ctx, tx, command.TenantID, command.ActorSubjectID, command.AccountID)
		if err != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_metadata_update", command.AccountID, err, &committedDenial)
		}
		if err := aggregate.UpdateMetadata(accountdomain.Metadata{DisplayName: command.DisplayName, ExternalReference: command.Reference, Category: command.Category}, command.ExpectedVersion, envelope.OccurredAt); err != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_metadata_update", command.AccountID, err, &committedDenial)
		}
		if err := updateAccountRow(ctx, tx, aggregate, command.ExpectedVersion); err != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_metadata_update", command.AccountID, err, &committedDenial)
		}
		result = accountCommandResult(aggregate, available, ledger)
		if err := insertAccountAudit(ctx, tx, envelope, aggregate.ID, "account.metadata_updated", "succeeded", map[string]string{"status": string(aggregate.Status), "category": aggregate.Category}); err != nil {
			return err
		}
		if err := insertAccountOutbox(ctx, tx, envelope, aggregate, "account.metadata.updated.v1"); err != nil {
			return err
		}
		if err := storeAccountOutcome(ctx, tx, envelope, result, 200); err != nil {
			return err
		}
		submission = accounts.CommandSubmission{Result: result}
		return nil
	})
	if err != nil {
		return submission, r.classifyUncommittedError(ctx, envelope, "account_metadata_update", command.AccountID, err)
	}
	return submission, committedDenial
}

func (r *AccountCommandRepository) ChangeStatus(ctx context.Context, command accounts.ChangeAccountStatusCommand, fingerprint [sha256.Size]byte) (submission accounts.CommandSubmission, err error) {
	if command.OccurredAt.IsZero() {
		command.OccurredAt = r.clock().UTC()
	}
	envelope := commandEnvelope{command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey, accounts.UpdateOperation, command.OccurredAt.UTC()}
	var committedDenial error
	err = WithSerializableSequence(ctx, r.database, "account-update|"+strings.ToLower(command.TenantID)+"|"+strings.ToLower(command.AccountID), 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, envelope.TenantID, envelope.ActorID); err != nil {
			return err
		}
		result, replay, replayedDenial, err := reserveAccountCommand(ctx, tx, envelope, fingerprint)
		if err != nil {
			return err
		}
		if replay {
			submission = accounts.CommandSubmission{Result: result, Replayed: true}
			committedDenial = replayedDenial
			return nil
		}
		aggregate, available, ledger, err := lockOwnedAccount(ctx, tx, command.TenantID, command.ActorSubjectID, command.AccountID)
		if err != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_status_change", command.AccountID, err, &committedDenial)
		}
		switch command.TargetStatus {
		case accountdomain.StatusFrozen:
			err = aggregate.Freeze(command.ExpectedVersion, envelope.OccurredAt)
		case accountdomain.StatusActive:
			err = aggregate.Reactivate(command.ExpectedVersion, envelope.OccurredAt)
		case accountdomain.StatusClosed:
			state, stateErr := authoritativeCloseState(ctx, tx, aggregate.ID, available, ledger)
			if stateErr != nil {
				return persistAccountDenial(ctx, tx, envelope, "account_status_change", command.AccountID, stateErr, &committedDenial)
			}
			err = aggregate.Close(command.ExpectedVersion, state, envelope.OccurredAt)
		default:
			err = fmt.Errorf("%w: unsupported target status", accountdomain.ErrInvalidTransition)
		}
		if err != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_status_change", command.AccountID, err, &committedDenial)
		}
		if err := updateAccountRow(ctx, tx, aggregate, command.ExpectedVersion); err != nil {
			return persistAccountDenial(ctx, tx, envelope, "account_status_change", command.AccountID, err, &committedDenial)
		}
		result = accountCommandResult(aggregate, available, ledger)
		if err := insertAccountAudit(ctx, tx, envelope, aggregate.ID, "account.status_changed", "succeeded", map[string]string{"status": string(aggregate.Status)}); err != nil {
			return err
		}
		if err := insertAccountOutbox(ctx, tx, envelope, aggregate, "account.status.changed.v1"); err != nil {
			return err
		}
		if err := storeAccountOutcome(ctx, tx, envelope, result, 200); err != nil {
			return err
		}
		submission = accounts.CommandSubmission{Result: result}
		return nil
	})
	if err != nil {
		return submission, r.classifyUncommittedError(ctx, envelope, "account_status_change", command.AccountID, err)
	}
	return submission, committedDenial
}
