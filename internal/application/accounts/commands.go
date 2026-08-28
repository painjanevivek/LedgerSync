package accounts

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
)

const (
	CreateOperation         = "accounts.create.v1"
	UpdateOperation         = "accounts.update.v1"
	MaxLifecycleReasonRunes = 256
)

var (
	ErrInvalidCommand         = errors.New("invalid account command")
	ErrAccountConflict        = errors.New("account conflicts with existing state")
	ErrInvalidTransition      = accountdomain.ErrInvalidTransition
	ErrNonZeroClose           = accountdomain.ErrNonZeroBalance
	ErrTerminalStatus         = accountdomain.ErrTerminalStatus
	ErrVersionConflict        = accountdomain.ErrVersionConflict
	ErrFinancialUnavailable   = accountdomain.ErrFinancialStateUnavailable
	ErrOperationalObligations = accountdomain.ErrOperationalObligations
	ErrIdempotencyConflict    = idempotency.ErrConflict
	ErrCommandInProgress      = idempotency.ErrInProgress
	ErrCommandUnavailable     = errors.New("account command dependency is temporarily unavailable")
)

type CreateAccountCommand struct {
	TenantID       string
	ActorSubjectID string
	CorrelationID  string
	IdempotencyKey string
	DisplayName    string
	Reference      string
	Category       string
	Currency       string
	OccurredAt     time.Time
}

type UpdateAccountMetadataCommand struct {
	TenantID        string
	ActorSubjectID  string
	CorrelationID   string
	IdempotencyKey  string
	AccountID       string
	ExpectedVersion int64
	DisplayName     string
	Reference       string
	Category        string
	OccurredAt      time.Time
}

type ChangeAccountStatusCommand struct {
	TenantID        string
	ActorSubjectID  string
	CorrelationID   string
	IdempotencyKey  string
	AccountID       string
	ExpectedVersion int64
	TargetStatus    accountdomain.Status
	Reason          string
	OccurredAt      time.Time
}

// CommandResult is persisted verbatim for replay. Financial values use
// canonical decimal strings even though newly created values are exactly zero.
type CommandResult struct {
	AccountID      string `json:"account_id"`
	TenantID       string `json:"tenant_id"`
	Currency       string `json:"currency"`
	Status         string `json:"status"`
	DisplayName    string `json:"display_name"`
	Reference      string `json:"reference"`
	Category       string `json:"category"`
	Version        string `json:"version"`
	AvailableMinor string `json:"available_minor"`
	LedgerMinor    string `json:"ledger_minor"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type CommandSubmission struct {
	Result   CommandResult
	Replayed bool
}

type CommandRepository interface {
	Create(context.Context, CreateAccountCommand, [sha256.Size]byte) (CommandSubmission, error)
	UpdateMetadata(context.Context, UpdateAccountMetadataCommand, [sha256.Size]byte) (CommandSubmission, error)
	ChangeStatus(context.Context, ChangeAccountStatusCommand, [sha256.Size]byte) (CommandSubmission, error)
}

type CommandService struct {
	repository CommandRepository
	clock      func() time.Time
}

func NewCommandService(repository CommandRepository, clock func() time.Time) (*CommandService, error) {
	if repository == nil {
		return nil, errors.New("account command repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &CommandService{repository: repository, clock: clock}, nil
}

func (s *CommandService) Create(ctx context.Context, command CreateAccountCommand) (CommandSubmission, error) {
	command = normalizeCreate(command)
	metadata, err := validateCommon(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey, command.DisplayName, command.Reference, command.Category)
	if err != nil {
		return CommandSubmission{}, err
	}
	if command.Currency != "INR" {
		return CommandSubmission{}, fmt.Errorf("%w: new local accounts must use INR", ErrInvalidCommand)
	}
	command.DisplayName, command.Reference, command.Category = metadata.DisplayName, metadata.ExternalReference, metadata.Category
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	} else {
		command.OccurredAt = command.OccurredAt.UTC()
	}
	fingerprint := idempotency.Fingerprint(command.TenantID, command.ActorSubjectID, CreateOperation, command.DisplayName, command.Reference, command.Category, command.Currency)
	return s.repository.Create(ctx, command, fingerprint)
}

func (s *CommandService) UpdateMetadata(ctx context.Context, command UpdateAccountMetadataCommand) (CommandSubmission, error) {
	command = normalizeMetadataUpdate(command)
	metadata, err := validateCommon(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey, command.DisplayName, command.Reference, command.Category)
	if err != nil || command.AccountID == "" || command.ExpectedVersion < 1 {
		if err != nil {
			return CommandSubmission{}, err
		}
		return CommandSubmission{}, fmt.Errorf("%w: account ID and positive expected version are required", ErrInvalidCommand)
	}
	command.DisplayName, command.Reference, command.Category = metadata.DisplayName, metadata.ExternalReference, metadata.Category
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	} else {
		command.OccurredAt = command.OccurredAt.UTC()
	}
	fingerprint := idempotency.Fingerprint(command.TenantID, command.ActorSubjectID, UpdateOperation, "metadata", command.AccountID, strconv.FormatInt(command.ExpectedVersion, 10), command.DisplayName, command.Reference, command.Category)
	return s.repository.UpdateMetadata(ctx, command, fingerprint)
}

func (s *CommandService) ChangeStatus(ctx context.Context, command ChangeAccountStatusCommand) (CommandSubmission, error) {
	command = normalizeStatusChange(command)
	if err := validateEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey); err != nil {
		return CommandSubmission{}, err
	}
	if command.AccountID == "" || command.ExpectedVersion < 1 || command.TargetStatus != accountdomain.StatusActive && command.TargetStatus != accountdomain.StatusFrozen && command.TargetStatus != accountdomain.StatusClosed {
		return CommandSubmission{}, fmt.Errorf("%w: account ID, positive expected version, and valid target status are required", ErrInvalidCommand)
	}
	if err := validateLifecycleReason(command.Reason); err != nil {
		return CommandSubmission{}, err
	}
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	} else {
		command.OccurredAt = command.OccurredAt.UTC()
	}
	fingerprint := idempotency.Fingerprint(command.TenantID, command.ActorSubjectID, UpdateOperation, "status", command.AccountID, strconv.FormatInt(command.ExpectedVersion, 10), string(command.TargetStatus), command.Reason)
	return s.repository.ChangeStatus(ctx, command, fingerprint)
}

func validateCommon(tenantID, actorID, correlationID, key, name, reference, category string) (accountdomain.Metadata, error) {
	if err := validateEnvelope(tenantID, actorID, correlationID, key); err != nil {
		return accountdomain.Metadata{}, err
	}
	metadata, err := accountdomain.NormalizeMetadata(accountdomain.Metadata{DisplayName: name, ExternalReference: reference, Category: category})
	if err != nil {
		return accountdomain.Metadata{}, fmt.Errorf("%w: %v", ErrInvalidCommand, err)
	}
	return metadata, nil
}

func validateEnvelope(tenantID, actorID, correlationID, key string) error {
	if tenantID == "" || actorID == "" || correlationID == "" {
		return fmt.Errorf("%w: tenant, actor, and correlation identifiers are required", ErrInvalidCommand)
	}
	if err := idempotency.ValidateKey(key); err != nil {
		return err
	}
	return nil
}

func normalizeCreate(command CreateAccountCommand) CreateAccountCommand {
	command.TenantID = strings.TrimSpace(command.TenantID)
	command.ActorSubjectID = strings.TrimSpace(command.ActorSubjectID)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.Currency = strings.ToUpper(strings.TrimSpace(command.Currency))
	return command
}

func normalizeMetadataUpdate(command UpdateAccountMetadataCommand) UpdateAccountMetadataCommand {
	command.TenantID = strings.TrimSpace(command.TenantID)
	command.ActorSubjectID = strings.TrimSpace(command.ActorSubjectID)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.AccountID = strings.TrimSpace(command.AccountID)
	return command
}

func normalizeStatusChange(command ChangeAccountStatusCommand) ChangeAccountStatusCommand {
	command.TenantID = strings.TrimSpace(command.TenantID)
	command.ActorSubjectID = strings.TrimSpace(command.ActorSubjectID)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.AccountID = strings.TrimSpace(command.AccountID)
	command.TargetStatus = accountdomain.Status(strings.ToLower(strings.TrimSpace(string(command.TargetStatus))))
	command.Reason = strings.TrimSpace(command.Reason)
	return command
}

func validateLifecycleReason(reason string) error {
	if !utf8.ValidString(reason) || utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > MaxLifecycleReasonRunes {
		return fmt.Errorf("%w: lifecycle reason must contain 1-%d Unicode characters", ErrInvalidCommand, MaxLifecycleReasonRunes)
	}
	for _, character := range reason {
		if unicode.IsControl(character) {
			return fmt.Errorf("%w: lifecycle reason contains a control character", ErrInvalidCommand)
		}
	}
	return nil
}
