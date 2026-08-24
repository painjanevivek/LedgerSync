package reconciliation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
)

const RunOperation = "reconciliation.run.v1"

var (
	ErrInvalidCommand      = errors.New("invalid reconciliation command")
	ErrAlreadyRunning      = errors.New("reconciliation already running")
	ErrIdempotencyConflict = idempotency.ErrConflict
	ErrCommandInProgress   = idempotency.ErrInProgress
	ErrCommandUnavailable  = errors.New("reconciliation command dependency is temporarily unavailable")
	ErrResponseUnknown     = errors.New("reconciliation command response is unknown")
)

type RunCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey string
	OccurredAt                                              time.Time
}

type CommandSubmission struct {
	Result      Result
	Replayed    bool
	Denial      string
	ActiveRunID string
}

type CommandRepository interface {
	RunCommand(context.Context, RunCommand, [sha256.Size]byte) (CommandSubmission, error)
}

type CommandService struct {
	repository CommandRepository
	clock      func() time.Time
}

func NewCommandService(repository CommandRepository, clock func() time.Time) (*CommandService, error) {
	if repository == nil {
		return nil, errors.New("reconciliation command repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &CommandService{repository: repository, clock: clock}, nil
}

func (s *CommandService) Run(ctx context.Context, command RunCommand) (CommandSubmission, error) {
	command.TenantID = strings.TrimSpace(command.TenantID)
	command.ActorSubjectID = strings.TrimSpace(command.ActorSubjectID)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.TenantID == "" || command.ActorSubjectID == "" || command.CorrelationID == "" {
		return CommandSubmission{}, fmt.Errorf("%w: tenant, actor, and correlation identifiers are required", ErrInvalidCommand)
	}
	if err := idempotency.ValidateKey(command.IdempotencyKey); err != nil {
		return CommandSubmission{}, err
	}
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	} else {
		command.OccurredAt = command.OccurredAt.UTC()
	}
	fingerprint := idempotency.Fingerprint(command.TenantID, command.ActorSubjectID, RunOperation)
	return s.repository.RunCommand(ctx, command, fingerprint)
}
