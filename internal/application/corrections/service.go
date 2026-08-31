// Package corrections defines the authenticated application contract for
// additive transfer corrections. A correction never edits an original transfer;
// it can only authorize one exact compensating transfer.
package corrections

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
)

var (
	ErrInvalidCommand = errors.New("invalid correction command")
	ErrNotFound       = errors.New("correction not found")
	ErrConflict       = errors.New("correction conflict")
	ErrForbidden      = errors.New("correction forbidden")
	ErrExpired        = errors.New("correction approval expired")
	ErrStepUpRequired = errors.New("recent step-up authentication required")
)

var reasonCodes = map[string]struct{}{
	"duplicate": {}, "wrong_destination": {}, "wrong_amount": {},
	"customer_request": {}, "operational_error": {}, "compliance_reversal": {},
}

type RequestCommand struct {
	TenantID, ActorSubjectID, OriginalTransferID string
	ReasonCode, OperatorNote, IdempotencyKey     string
	CorrelationID                                string
	StepUpAuthenticatedAt                        time.Time
	RequestedAt                                  time.Time
}

type DecisionCommand struct {
	TenantID, ActorSubjectID, CorrectionID string
	Reason                                 string
	CorrelationID                          string
	StepUpAuthenticatedAt                  time.Time
	DecidedAt                              time.Time
}

type CancelCommand struct {
	TenantID, ActorSubjectID, CorrectionID string
	Reason, CorrelationID                  string
	CancelledAt                            time.Time
}

type PostCommand struct {
	TenantID, ActorSubjectID, CorrectionID string
	IdempotencyKey, CorrelationID          string
	StepUpAuthenticatedAt                  time.Time
	OccurredAt                             time.Time
}

type Event struct {
	CorrectionID           string `json:"correction_id"`
	OriginalTransferID     string `json:"original_transfer_id"`
	CompensationTransferID string `json:"compensation_transfer_id,omitempty"`
	OriginalJournalID      string `json:"original_journal_id"`
	CompensationJournalID  string `json:"compensation_journal_id,omitempty"`
	RequesterSubjectID     string `json:"requester_subject_id"`
	ApproverSubjectID      string `json:"approver_subject_id,omitempty"`
	DebitAccountID         string `json:"debit_account_id"`
	CreditAccountID        string `json:"credit_account_id"`
	AmountMinor            string `json:"amount_minor"`
	Currency               string `json:"currency"`
	ReasonCode             string `json:"reason_code"`
	OperatorNote           string `json:"operator_note"`
	DecisionReason         string `json:"decision_reason,omitempty"`
	Status                 string `json:"status"`
	PolicyVersion          string `json:"policy_version"`
	ControlMode            string `json:"control_mode"`
	StepUpRequired         bool   `json:"step_up_required"`
	ApprovalExpiresAt      string `json:"approval_expires_at"`
	RequestedAt            string `json:"requested_at"`
	UpdatedAt              string `json:"updated_at"`
}

type Submission struct {
	Event    Event `json:"event"`
	Replayed bool  `json:"replayed"`
}

type Query struct {
	Status, Cursor string
	Limit          int
}
type Page struct {
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Repository interface {
	Request(context.Context, RequestCommand, [sha256.Size]byte) (Submission, error)
	Approve(context.Context, DecisionCommand) (Event, error)
	Reject(context.Context, DecisionCommand) (Event, error)
	Cancel(context.Context, CancelCommand) (Event, error)
	Post(context.Context, PostCommand) (Submission, error)
	Get(context.Context, string, string, string) (Event, error)
	List(context.Context, string, string, Query) (Page, error)
}

type Service struct {
	repository Repository
	clock      func() time.Time
}

func NewService(repository Repository, clock func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, errors.New("correction repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository: repository, clock: clock}, nil
}

func (s *Service) Request(ctx context.Context, command RequestCommand) (Submission, error) {
	command.TenantID, command.ActorSubjectID, command.OriginalTransferID = trim(command.TenantID), trim(command.ActorSubjectID), trim(command.OriginalTransferID)
	command.ReasonCode, command.OperatorNote, command.IdempotencyKey, command.CorrelationID = trim(command.ReasonCode), trim(command.OperatorNote), trim(command.IdempotencyKey), trim(command.CorrelationID)
	if command.RequestedAt.IsZero() {
		command.RequestedAt = s.clock().UTC()
	}
	if command.TenantID == "" || command.ActorSubjectID == "" || command.OriginalTransferID == "" || command.OperatorNote == "" || len(command.OperatorNote) > 500 || len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 255 || command.CorrelationID == "" || !ValidReasonCode(command.ReasonCode) {
		return Submission{}, ErrInvalidCommand
	}
	fingerprint := sha256.Sum256([]byte(strings.Join([]string{command.TenantID, command.ActorSubjectID, command.OriginalTransferID, command.ReasonCode, command.OperatorNote}, "\x00")))
	return s.repository.Request(ctx, command, fingerprint)
}

func (s *Service) Approve(ctx context.Context, command DecisionCommand) (Event, error) {
	return s.decide(ctx, command, true)
}
func (s *Service) Reject(ctx context.Context, command DecisionCommand) (Event, error) {
	return s.decide(ctx, command, false)
}
func (s *Service) decide(ctx context.Context, command DecisionCommand, approve bool) (Event, error) {
	command.TenantID, command.ActorSubjectID, command.CorrectionID, command.Reason, command.CorrelationID = trim(command.TenantID), trim(command.ActorSubjectID), trim(command.CorrectionID), trim(command.Reason), trim(command.CorrelationID)
	if command.DecidedAt.IsZero() {
		command.DecidedAt = s.clock().UTC()
	}
	if command.TenantID == "" || command.ActorSubjectID == "" || command.CorrectionID == "" || command.Reason == "" || len(command.Reason) > 500 || command.CorrelationID == "" {
		return Event{}, ErrInvalidCommand
	}
	if approve {
		return s.repository.Approve(ctx, command)
	}
	return s.repository.Reject(ctx, command)
}

func (s *Service) Cancel(ctx context.Context, command CancelCommand) (Event, error) {
	command.TenantID, command.ActorSubjectID, command.CorrectionID, command.Reason, command.CorrelationID = trim(command.TenantID), trim(command.ActorSubjectID), trim(command.CorrectionID), trim(command.Reason), trim(command.CorrelationID)
	if command.CancelledAt.IsZero() {
		command.CancelledAt = s.clock().UTC()
	}
	if command.TenantID == "" || command.ActorSubjectID == "" || command.CorrectionID == "" || command.Reason == "" || len(command.Reason) > 500 || command.CorrelationID == "" {
		return Event{}, ErrInvalidCommand
	}
	return s.repository.Cancel(ctx, command)
}

func (s *Service) Post(ctx context.Context, command PostCommand) (Submission, error) {
	command.TenantID, command.ActorSubjectID, command.CorrectionID, command.IdempotencyKey, command.CorrelationID = trim(command.TenantID), trim(command.ActorSubjectID), trim(command.CorrectionID), trim(command.IdempotencyKey), trim(command.CorrelationID)
	if command.OccurredAt.IsZero() {
		command.OccurredAt = s.clock().UTC()
	}
	if command.TenantID == "" || command.ActorSubjectID == "" || command.CorrectionID == "" || command.CorrelationID == "" || idempotency.ValidateKey(command.IdempotencyKey) != nil {
		return Submission{}, ErrInvalidCommand
	}
	return s.repository.Post(ctx, command)
}

func (s *Service) Get(ctx context.Context, tenantID, actorID, correctionID string) (Event, error) {
	tenantID, actorID, correctionID = trim(tenantID), trim(actorID), trim(correctionID)
	if tenantID == "" || actorID == "" || correctionID == "" {
		return Event{}, ErrInvalidCommand
	}
	return s.repository.Get(ctx, tenantID, actorID, correctionID)
}
func (s *Service) List(ctx context.Context, tenantID, actorID string, query Query) (Page, error) {
	tenantID, actorID, query.Status, query.Cursor = trim(tenantID), trim(actorID), trim(query.Status), trim(query.Cursor)
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}
	if tenantID == "" || actorID == "" || !validStatus(query.Status) {
		return Page{}, ErrInvalidCommand
	}
	return s.repository.List(ctx, tenantID, actorID, query)
}

func ValidReasonCode(value string) bool { _, ok := reasonCodes[trim(value)]; return ok }
func validStatus(value string) bool {
	switch value {
	case "", "requested", "approved", "rejected", "cancelled", "expired", "posted":
		return true
	}
	return false
}
func trim(value string) string { return strings.TrimSpace(value) }
