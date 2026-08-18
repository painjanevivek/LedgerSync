// Package transfer models a customer-visible request and its irreversible
// outcome. Persistence supplies the transactionality and idempotency boundary.
package transfer

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var ErrInvalidTransfer = errors.New("invalid transfer")

type Status string

const (
	StatusPending  Status = "pending"
	StatusPosted   Status = "posted"
	StatusRejected Status = "rejected"
)

type Transfer struct {
	ID                   string
	TenantID             string
	ActorID              string
	DebitAccountID       string
	CreditAccountID      string
	Amount               money.Money
	Status               Status
	RejectionCode        string
	CreatedAt            time.Time
	CompletedAt          *time.Time
	JournalTransactionID string
}

func New(id, tenantID, actorID, debitAccountID, creditAccountID string, amount money.Money, now time.Time) (Transfer, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return Transfer{}, fmt.Errorf("%w: id, tenant, and actor are required", ErrInvalidTransfer)
	}
	if strings.TrimSpace(debitAccountID) == "" || strings.TrimSpace(creditAccountID) == "" || debitAccountID == creditAccountID {
		return Transfer{}, fmt.Errorf("%w: distinct debit and credit accounts are required", ErrInvalidTransfer)
	}
	if !amount.IsPositive() {
		return Transfer{}, fmt.Errorf("%w: amount must be positive", ErrInvalidTransfer)
	}
	return Transfer{
		ID: id, TenantID: tenantID, ActorID: actorID, DebitAccountID: debitAccountID,
		CreditAccountID: creditAccountID, Amount: amount, Status: StatusPending, CreatedAt: now.UTC(),
	}, nil
}

func (t *Transfer) Post(journalTransactionID string, completedAt time.Time) error {
	if t.Status != StatusPending || strings.TrimSpace(journalTransactionID) == "" {
		return fmt.Errorf("%w: only a pending transfer can be posted once", ErrInvalidTransfer)
	}
	at := completedAt.UTC()
	t.Status = StatusPosted
	t.JournalTransactionID = journalTransactionID
	t.CompletedAt = &at
	return nil
}

func (t *Transfer) Reject(code string, completedAt time.Time) error {
	if t.Status != StatusPending || strings.TrimSpace(code) == "" {
		return fmt.Errorf("%w: only a pending transfer can be rejected once", ErrInvalidTransfer)
	}
	at := completedAt.UTC()
	t.Status = StatusRejected
	t.RejectionCode = code
	t.CompletedAt = &at
	return nil
}
