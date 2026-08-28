// Package funding models controlled, non-custodial evidence that authorized
// external value was recorded in LedgerSync. It does not model bank settlement.
package funding

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var (
	ErrInvalidFunding              = errors.New("invalid funding event")
	ErrInvalidTransition           = errors.New("invalid funding transition")
	ErrIndependentApprovalRequired = errors.New("independent funding approval required")
)

type Status string

const (
	StatusRequested   Status = "requested"
	StatusApproved    Status = "approved"
	StatusPosted      Status = "posted"
	StatusRejected    Status = "rejected"
	StatusCompensated Status = "compensated"
)

type RequestInput struct {
	ID                   string
	TenantID             string
	RequesterSubjectID   string
	DestinationAccountID string
	SystemAccountID      string
	ExternalReference    string
	EvidenceReference    string
	IdempotencyKey       string
	CorrelationID        string
	Amount               money.Money
}

type Event struct {
	ID                   string
	TenantID             string
	RequesterSubjectID   string
	ApproverSubjectID    string
	DestinationAccountID string
	SystemAccountID      string
	ExternalReference    string
	EvidenceReference    string
	IdempotencyKey       string
	CorrelationID        string
	Amount               money.Money
	Status               Status
	DecisionReason       string
	DemoPolicy           bool
	JournalTransactionID string
	CompensationEventID  string
	RequestedAt          time.Time
	ApprovedAt           *time.Time
	PostedAt             *time.Time
	RejectedAt           *time.Time
	CompensatedAt        *time.Time
}

func Request(input RequestInput, requestedAt time.Time) (Event, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.RequesterSubjectID = strings.TrimSpace(input.RequesterSubjectID)
	input.DestinationAccountID = strings.TrimSpace(input.DestinationAccountID)
	input.SystemAccountID = strings.TrimSpace(input.SystemAccountID)
	input.ExternalReference = strings.TrimSpace(input.ExternalReference)
	input.EvidenceReference = strings.TrimSpace(input.EvidenceReference)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.CorrelationID = strings.TrimSpace(input.CorrelationID)
	if input.ID == "" || input.TenantID == "" || input.RequesterSubjectID == "" || input.DestinationAccountID == "" || input.SystemAccountID == "" || input.DestinationAccountID == input.SystemAccountID {
		return Event{}, fmt.Errorf("%w: identities and distinct accounts are required", ErrInvalidFunding)
	}
	if input.ExternalReference == "" || input.EvidenceReference == "" || len(input.IdempotencyKey) < 16 || input.CorrelationID == "" || !input.Amount.IsPositive() {
		return Event{}, fmt.Errorf("%w: exact amount, evidence, references, correlation, and idempotency are required", ErrInvalidFunding)
	}
	return Event{
		ID: input.ID, TenantID: input.TenantID, RequesterSubjectID: input.RequesterSubjectID,
		DestinationAccountID: input.DestinationAccountID, SystemAccountID: input.SystemAccountID,
		ExternalReference: input.ExternalReference, EvidenceReference: input.EvidenceReference,
		IdempotencyKey: input.IdempotencyKey, CorrelationID: input.CorrelationID, Amount: input.Amount,
		Status: StatusRequested, RequestedAt: requestedAt.UTC(),
	}, nil
}

func (e *Event) Approve(approverSubjectID, reason string, demoPolicy bool, approvedAt time.Time) error {
	approverSubjectID, reason = strings.TrimSpace(approverSubjectID), strings.TrimSpace(reason)
	if e.Status != StatusRequested || approverSubjectID == "" || reason == "" {
		return ErrInvalidTransition
	}
	if !demoPolicy && approverSubjectID == e.RequesterSubjectID {
		return ErrIndependentApprovalRequired
	}
	at := approvedAt.UTC()
	e.Status, e.ApproverSubjectID, e.DecisionReason, e.DemoPolicy, e.ApprovedAt = StatusApproved, approverSubjectID, reason, demoPolicy, &at
	return nil
}

func (e *Event) Reject(reason string, rejectedAt time.Time) error {
	reason = strings.TrimSpace(reason)
	if e.Status != StatusRequested || reason == "" {
		return ErrInvalidTransition
	}
	at := rejectedAt.UTC()
	e.Status, e.DecisionReason, e.RejectedAt = StatusRejected, reason, &at
	return nil
}

func (e *Event) Post(journalTransactionID string, postedAt time.Time) error {
	journalTransactionID = strings.TrimSpace(journalTransactionID)
	if e.Status != StatusApproved || journalTransactionID == "" {
		return ErrInvalidTransition
	}
	at := postedAt.UTC()
	e.Status, e.JournalTransactionID, e.PostedAt = StatusPosted, journalTransactionID, &at
	return nil
}

// MarkCompensated links the immutable original to a separate, additive
// funding event whose balanced journal reverses the original movement.
func (e *Event) MarkCompensated(compensationEventID string, compensatedAt time.Time) error {
	compensationEventID = strings.TrimSpace(compensationEventID)
	if e.Status != StatusPosted || compensationEventID == "" || e.JournalTransactionID == "" {
		return ErrInvalidTransition
	}
	at := compensatedAt.UTC()
	e.Status, e.CompensationEventID, e.CompensatedAt = StatusCompensated, compensationEventID, &at
	return nil
}
