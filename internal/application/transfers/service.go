// Package transfers coordinates the exact, idempotent transfer command. The
// persistence adapter owns SQL and transaction mechanics; this package owns
// the application contract and never depends on a database implementation.
package transfers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var ErrInvalidCommand = errors.New("invalid transfer command")

// Command is the authenticated, canonical financial intent. Amount has
// already been parsed from decimal text without using floating point.
type Command struct {
	TenantID        string
	ActorSubjectID  string
	DebitAccountID  string
	CreditAccountID string
	Amount          money.Money
	IdempotencyKey  string
	CorrelationID   string
	OccurredAt      time.Time
}

// Balance is the committed projection returned with a posted transfer. It is
// deliberately an exact minor-unit value, not a display-formatted decimal.
type Balance struct {
	AccountID   string `json:"account_id"`
	Currency    string `json:"currency"`
	PostedMinor int64  `json:"posted_minor"`
	Version     int64  `json:"version"`
	AsOf        string `json:"as_of"`
}

// Result is persisted verbatim as the idempotency outcome. Keeping the result
// stable makes a retry after a lost response indistinguishable from the first
// successful response to a client.
type Result struct {
	TransferID             string             `json:"transfer_id"`
	Status                 string             `json:"status"`
	Currency               string             `json:"currency"`
	AmountMinor            int64              `json:"amount_minor"`
	OccurredAt             string             `json:"occurred_at"`
	MinimumBalanceVersions map[string]int64   `json:"minimum_balance_versions"`
	Balances               map[string]Balance `json:"balances,omitempty"`
	RejectionCode          string             `json:"rejection_code,omitempty"`
}

// Submission distinguishes a completed original request from a stable
// idempotency replay without changing the persisted business result.
type Submission struct {
	Result   Result
	Replayed bool
}

// Repository is the single financial transaction boundary. Implementations
// must reserve/resolve idempotency, lock account projections in stable order,
// and commit every financial side effect atomically.
type Repository interface {
	Submit(context.Context, Command, [sha256.Size]byte) (Submission, error)
}

type Service struct {
	repository Repository
	clock      func() time.Time
	observers  []OutcomeObserver
}

func NewService(repository Repository, clock func() time.Time, observers ...OutcomeObserver) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("transfer repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository: repository, clock: clock, observers: observers}, nil
}

func (s *Service) Submit(ctx context.Context, command Command) (Submission, error) {
	command = normalize(command)
	if err := validateCommand(command); err != nil {
		return Submission{}, err
	}
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	}
	fingerprint, err := Fingerprint(IdempotencyRequest{
		TenantID:        command.TenantID,
		ActorSubjectID:  command.ActorSubjectID,
		Operation:       transferOperation,
		Key:             command.IdempotencyKey,
		DebitAccountID:  command.DebitAccountID,
		CreditAccountID: command.CreditAccountID,
		Amount:          command.Amount,
	})
	if err != nil {
		return Submission{}, err
	}
	submission, err := s.repository.Submit(ctx, command, fingerprint)
	if err != nil {
		for _, observer := range s.observers {
			if observer != nil {
				observer.ObserveFailure(err)
			}
		}
		return Submission{}, err
	}
	for _, observer := range s.observers {
		if observer != nil {
			observer.ObserveSubmission(submission.Result, submission.Replayed)
		}
	}
	return submission, nil
}

func normalize(command Command) Command {
	command.TenantID = strings.TrimSpace(command.TenantID)
	command.ActorSubjectID = strings.TrimSpace(command.ActorSubjectID)
	command.DebitAccountID = strings.TrimSpace(command.DebitAccountID)
	command.CreditAccountID = strings.TrimSpace(command.CreditAccountID)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	return command
}

func validateCommand(command Command) error {
	if command.TenantID == "" || command.ActorSubjectID == "" || command.DebitAccountID == "" || command.CreditAccountID == "" || command.DebitAccountID == command.CreditAccountID {
		return fmt.Errorf("%w: tenant, actor, and distinct account identifiers are required", ErrInvalidCommand)
	}
	if !command.Amount.IsPositive() {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidCommand)
	}
	if err := ValidateKey(command.IdempotencyKey); err != nil {
		return err
	}
	return nil
}
