// Package funding coordinates controlled funding commands without coupling the
// application contract to PostgreSQL or HTTP.
package funding

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var (
	ErrInvalidCommand = errors.New("invalid funding command")
	ErrNotFound       = errors.New("funding event not found")
	ErrConflict       = errors.New("funding command conflict")
	ErrForbidden      = errors.New("funding command forbidden")
	ErrLimitExceeded  = errors.New("funding limit exceeded")
)

type PolicyMode string

const (
	PolicyProductionDualControl   PolicyMode = "production_dual_control"
	PolicyLocalDemoSingleOperator PolicyMode = "local_demo_single_operator"
)

type RequestCommand struct {
	TenantID             string
	ActorSubjectID       string
	DestinationAccountID string
	Amount               money.Money
	ExternalReference    string
	EvidenceReference    string
	IdempotencyKey       string
	CorrelationID        string
	RequestedAt          time.Time
}

type DecisionCommand struct {
	TenantID       string
	ActorSubjectID string
	FundingEventID string
	Reason         string
	CorrelationID  string
	DecidedAt      time.Time
}

type ActionCommand struct {
	TenantID       string
	ActorSubjectID string
	FundingEventID string
	CorrelationID  string
	OccurredAt     time.Time
}

type CompensationCommand struct {
	TenantID       string
	ActorSubjectID string
	FundingEventID string
	ReasonCode     string
	OperatorNote   string
	IdempotencyKey string
	CorrelationID  string
	OccurredAt     time.Time
}

type Event struct {
	FundingEventID           string `json:"funding_event_id"`
	Status                   string `json:"status"`
	DestinationAccountID     string `json:"destination_account_id"`
	SystemAccountID          string `json:"system_account_id,omitempty"`
	Currency                 string `json:"currency"`
	AmountMinor              string `json:"amount_minor"`
	ExternalReference        string `json:"external_reference"`
	EvidenceReference        string `json:"evidence_reference"`
	RequesterSubjectID       string `json:"requester_subject_id"`
	ApproverSubjectID        string `json:"approver_subject_id,omitempty"`
	DecisionReason           string `json:"decision_reason,omitempty"`
	DemoPolicy               bool   `json:"demo_policy"`
	JournalTransactionID     string `json:"journal_transaction_id,omitempty"`
	CompensationOfEventID    string `json:"compensation_of_event_id,omitempty"`
	CompensationEventID      string `json:"compensation_event_id,omitempty"`
	CompensationReasonCode   string `json:"compensation_reason_code,omitempty"`
	CompensationOperatorNote string `json:"compensation_operator_note,omitempty"`
	RequestedAt              string `json:"requested_at"`
	UpdatedAt                string `json:"updated_at"`
	BalanceVersion           string `json:"balance_version,omitempty"`
}

type Submission struct {
	Event    Event `json:"event"`
	Replayed bool  `json:"replayed"`
}

type Query struct {
	Status string
	Cursor string
	Limit  int
}

type Page struct {
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Reconciliation struct {
	FundingEventID    string `json:"funding_event_id"`
	ExternalReference string `json:"external_reference"`
	Status            string `json:"status"`
	ExpectedMinor     string `json:"expected_minor"`
	PostedDebitMinor  string `json:"posted_debit_minor"`
	PostedCreditMinor string `json:"posted_credit_minor"`
	Currency          string `json:"currency"`
	CheckedAt         string `json:"checked_at"`
}

type Repository interface {
	Request(context.Context, RequestCommand, [sha256.Size]byte) (Submission, error)
	Approve(context.Context, DecisionCommand, bool) (Event, error)
	Reject(context.Context, DecisionCommand) (Event, error)
	Post(context.Context, ActionCommand) (Submission, error)
	Compensate(context.Context, CompensationCommand, [sha256.Size]byte) (Submission, error)
	Get(context.Context, string, string, string) (Event, error)
	List(context.Context, string, string, Query) (Page, error)
	Reconcile(context.Context, string, string, string) (Reconciliation, error)
}

type Service struct {
	repository Repository
	policy     PolicyMode
	clock      func() time.Time
}

func NewService(repository Repository, policy PolicyMode, clock func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, errors.New("funding repository is required")
	}
	if policy != PolicyProductionDualControl && policy != PolicyLocalDemoSingleOperator {
		return nil, errors.New("recognized funding policy is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository: repository, policy: policy, clock: clock}, nil
}

func (s *Service) Request(ctx context.Context, command RequestCommand) (Submission, error) {
	command = normalizeRequest(command)
	if command.TenantID == "" || command.ActorSubjectID == "" || command.DestinationAccountID == "" || command.ExternalReference == "" || command.EvidenceReference == "" || len(command.IdempotencyKey) < 16 || command.CorrelationID == "" || !command.Amount.IsPositive() {
		return Submission{}, ErrInvalidCommand
	}
	if command.RequestedAt.IsZero() {
		command.RequestedAt = s.clock().UTC()
	}
	return s.repository.Request(ctx, command, requestFingerprint(command))
}

func (s *Service) Approve(ctx context.Context, command DecisionCommand) (Event, error) {
	command = s.normalizeDecision(command)
	if !validDecision(command) {
		return Event{}, ErrInvalidCommand
	}
	return s.repository.Approve(ctx, command, s.policy == PolicyLocalDemoSingleOperator)
}

func (s *Service) Reject(ctx context.Context, command DecisionCommand) (Event, error) {
	command = s.normalizeDecision(command)
	if !validDecision(command) {
		return Event{}, ErrInvalidCommand
	}
	return s.repository.Reject(ctx, command)
}

func (s *Service) Post(ctx context.Context, command ActionCommand) (Submission, error) {
	command = s.normalizeAction(command)
	if command.TenantID == "" || command.ActorSubjectID == "" || command.FundingEventID == "" || command.CorrelationID == "" {
		return Submission{}, ErrInvalidCommand
	}
	return s.repository.Post(ctx, command)
}

func (s *Service) Compensate(ctx context.Context, command CompensationCommand) (Submission, error) {
	command = s.normalizeCompensation(command)
	if command.TenantID == "" || command.ActorSubjectID == "" || command.FundingEventID == "" || command.ReasonCode == "" || command.OperatorNote == "" || len(command.IdempotencyKey) < 16 || command.CorrelationID == "" {
		return Submission{}, ErrInvalidCommand
	}
	return s.repository.Compensate(ctx, command, compensationFingerprint(command))
}

func (s *Service) Get(ctx context.Context, tenantID, actorID, eventID string) (Event, error) {
	return s.repository.Get(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(actorID), strings.TrimSpace(eventID))
}

func (s *Service) List(ctx context.Context, tenantID, actorID string, query Query) (Page, error) {
	tenantID, actorID, query.Status, query.Cursor = strings.TrimSpace(tenantID), strings.TrimSpace(actorID), strings.TrimSpace(query.Status), strings.TrimSpace(query.Cursor)
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}
	return s.repository.List(ctx, tenantID, actorID, query)
}

func (s *Service) Reconcile(ctx context.Context, tenantID, actorID, eventID string) (Reconciliation, error) {
	return s.repository.Reconcile(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(actorID), strings.TrimSpace(eventID))
}

func normalizeRequest(command RequestCommand) RequestCommand {
	command.TenantID, command.ActorSubjectID = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID)
	command.DestinationAccountID = strings.TrimSpace(command.DestinationAccountID)
	command.ExternalReference, command.EvidenceReference = strings.TrimSpace(command.ExternalReference), strings.TrimSpace(command.EvidenceReference)
	command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.IdempotencyKey), strings.TrimSpace(command.CorrelationID)
	return command
}

func (s *Service) normalizeDecision(command DecisionCommand) DecisionCommand {
	command.TenantID, command.ActorSubjectID, command.FundingEventID = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID), strings.TrimSpace(command.FundingEventID)
	command.Reason, command.CorrelationID = strings.TrimSpace(command.Reason), strings.TrimSpace(command.CorrelationID)
	if command.DecidedAt.IsZero() {
		command.DecidedAt = s.clock().UTC()
	}
	return command
}

func (s *Service) normalizeAction(command ActionCommand) ActionCommand {
	command.TenantID, command.ActorSubjectID, command.FundingEventID = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID), strings.TrimSpace(command.FundingEventID)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	}
	return command
}

func (s *Service) normalizeCompensation(command CompensationCommand) CompensationCommand {
	command.TenantID, command.ActorSubjectID, command.FundingEventID = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID), strings.TrimSpace(command.FundingEventID)
	command.ReasonCode, command.OperatorNote = strings.TrimSpace(command.ReasonCode), strings.TrimSpace(command.OperatorNote)
	command.IdempotencyKey, command.CorrelationID = strings.TrimSpace(command.IdempotencyKey), strings.TrimSpace(command.CorrelationID)
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	}
	return command
}

func validDecision(command DecisionCommand) bool {
	return command.TenantID != "" && command.ActorSubjectID != "" && command.FundingEventID != "" && command.Reason != "" && command.CorrelationID != ""
}

func requestFingerprint(command RequestCommand) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.Join([]string{command.TenantID, command.ActorSubjectID, command.DestinationAccountID, command.Amount.Currency().Code, strconv.FormatInt(command.Amount.Minor(), 10), command.ExternalReference, command.EvidenceReference}, "\x00")))
}

func compensationFingerprint(command CompensationCommand) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.Join([]string{command.TenantID, command.ActorSubjectID, command.FundingEventID, command.ReasonCode, command.OperatorNote}, "\x00")))
}

func ParseMinor(value string) (int64, error) {
	if value == "" || strings.TrimSpace(value) != value || (len(value) > 1 && value[0] == '0') {
		return 0, fmt.Errorf("%w: non-canonical integer", ErrInvalidCommand)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%w: minor units outside range", ErrInvalidCommand)
	}
	return parsed, nil
}
