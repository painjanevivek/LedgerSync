// Package transfers coordinates the exact, idempotent transfer command. The
// persistence adapter owns SQL and transaction mechanics; this package owns
// the application contract and never depends on a database implementation.
package transfers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var ErrInvalidCommand = errors.New("invalid transfer command")

// Command is the authenticated, canonical financial intent. Amount has
// already been parsed from decimal text without using floating point.
type Command struct {
	TenantID        identifier.UUID
	ActorSubjectID  string
	DebitAccountID  identifier.UUID
	CreditAccountID identifier.UUID
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

type resultJSON struct {
	TransferID             string                 `json:"transfer_id"`
	Status                 string                 `json:"status"`
	Currency               string                 `json:"currency"`
	AmountMinor            string                 `json:"amount_minor"`
	OccurredAt             string                 `json:"occurred_at"`
	MinimumBalanceVersions map[string]string      `json:"minimum_balance_versions"`
	Balances               map[string]balanceJSON `json:"balances,omitempty"`
	RejectionCode          string                 `json:"rejection_code,omitempty"`
}

type balanceJSON struct {
	AccountID   string `json:"account_id"`
	Currency    string `json:"currency"`
	PostedMinor string `json:"posted_minor"`
	Version     string `json:"version"`
	AsOf        string `json:"as_of"`
}

// MarshalJSON preserves signed-64-bit financial values across JavaScript by
// encoding them as canonical base-10 strings. Internal arithmetic stays int64.
func (r Result) MarshalJSON() ([]byte, error) {
	versions := make(map[string]string, len(r.MinimumBalanceVersions))
	for accountID, version := range r.MinimumBalanceVersions {
		versions[accountID] = strconv.FormatInt(version, 10)
	}
	balances := make(map[string]balanceJSON, len(r.Balances))
	for accountID, balance := range r.Balances {
		balances[accountID] = balanceJSON{AccountID: balance.AccountID, Currency: balance.Currency, PostedMinor: strconv.FormatInt(balance.PostedMinor, 10), Version: strconv.FormatInt(balance.Version, 10), AsOf: balance.AsOf}
	}
	return json.Marshal(resultJSON{TransferID: r.TransferID, Status: r.Status, Currency: r.Currency, AmountMinor: strconv.FormatInt(r.AmountMinor, 10), OccurredAt: r.OccurredAt, MinimumBalanceVersions: versions, Balances: balances, RejectionCode: r.RejectionCode})
}

// UnmarshalJSON accepts only the lossless string contract used by persisted
// idempotency outcomes and API responses; numeric JSON is deliberately rejected.
func (r *Result) UnmarshalJSON(data []byte) error {
	var encoded resultJSON
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}
	amount, err := parseFinancialInt(encoded.AmountMinor, false)
	if err != nil {
		return fmt.Errorf("decode amount_minor: %w", err)
	}
	versions := make(map[string]int64, len(encoded.MinimumBalanceVersions))
	for accountID, value := range encoded.MinimumBalanceVersions {
		version, err := parseFinancialInt(value, true)
		if err != nil {
			return fmt.Errorf("decode minimum balance version: %w", err)
		}
		versions[accountID] = version
	}
	balances := make(map[string]Balance, len(encoded.Balances))
	for accountID, value := range encoded.Balances {
		posted, err := parseFinancialInt(value.PostedMinor, true)
		if err != nil {
			return fmt.Errorf("decode posted_minor: %w", err)
		}
		version, err := parseFinancialInt(value.Version, true)
		if err != nil {
			return fmt.Errorf("decode balance version: %w", err)
		}
		balances[accountID] = Balance{AccountID: value.AccountID, Currency: value.Currency, PostedMinor: posted, Version: version, AsOf: value.AsOf}
	}
	*r = Result{TransferID: encoded.TransferID, Status: encoded.Status, Currency: encoded.Currency, AmountMinor: amount, OccurredAt: encoded.OccurredAt, MinimumBalanceVersions: versions, Balances: balances, RejectionCode: encoded.RejectionCode}
	return nil
}

func parseFinancialInt(value string, allowZero bool) (int64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') || strings.TrimSpace(value) != value {
		return 0, errors.New("non-canonical integer string")
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, errors.New("non-canonical integer string")
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 || (!allowZero && parsed == 0) {
		return 0, errors.New("integer string is outside the permitted range")
	}
	return parsed, nil
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
	command.ActorSubjectID = strings.TrimSpace(command.ActorSubjectID)
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
