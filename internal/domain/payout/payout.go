// Package payout defines the provider-led payout lifecycle. It has no network
// or storage dependency, so every transition can be tested independently of a
// provider implementation.
package payout

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var ErrInvalidPayout = errors.New("invalid payout")

type Status string

const (
	StatusRequested       Status = "requested"
	StatusReserved        Status = "reserved"
	StatusPendingApproval Status = "pending_approval"
	StatusApproved        Status = "approved"
	StatusDispatching     Status = "dispatching"
	StatusProviderPending Status = "provider_pending"
	StatusSettled         Status = "settled"
	StatusFailed          Status = "failed"
	StatusCancelled       Status = "cancelled"
	StatusExpired         Status = "expired"
	StatusReconciled      Status = "reconciled"
)

type Payout struct {
	ID, TenantID, SourceAccountID, BeneficiaryReference, RequesterID string
	Amount, Fee                                                      money.Money
	Status                                                           Status
	CreatedAt                                                        time.Time
}

func New(id, tenantID, sourceAccountID, beneficiaryReference, requesterID string, amount, fee money.Money, now time.Time) (Payout, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(sourceAccountID) == "" || strings.TrimSpace(beneficiaryReference) == "" || strings.TrimSpace(requesterID) == "" || !amount.IsPositive() || amount.Currency().Code != "INR" || fee.Currency() != amount.Currency() {
		return Payout{}, ErrInvalidPayout
	}
	return Payout{ID: id, TenantID: tenantID, SourceAccountID: sourceAccountID, BeneficiaryReference: beneficiaryReference, RequesterID: requesterID, Amount: amount, Fee: fee, Status: StatusRequested, CreatedAt: now.UTC()}, nil
}

func (p *Payout) Transition(next Status, actorID string) error {
	if strings.TrimSpace(actorID) == "" || !allowedTransition(p.Status, next) {
		return fmt.Errorf("%w: %s to %s", ErrInvalidPayout, p.Status, next)
	}
	if next == StatusApproved && actorID == p.RequesterID {
		return fmt.Errorf("%w: requester cannot approve", ErrInvalidPayout)
	}
	p.Status = next
	return nil
}

func allowedTransition(current, next Status) bool {
	allowed := map[Status][]Status{
		StatusRequested:       {StatusReserved, StatusCancelled, StatusExpired},
		StatusReserved:        {StatusPendingApproval, StatusCancelled, StatusExpired},
		StatusPendingApproval: {StatusApproved, StatusCancelled, StatusExpired},
		StatusApproved:        {StatusDispatching, StatusCancelled, StatusExpired},
		StatusDispatching:     {StatusProviderPending, StatusFailed},
		StatusProviderPending: {StatusSettled, StatusFailed, StatusCancelled, StatusExpired},
		StatusSettled:         {StatusReconciled},
		StatusFailed:          {StatusReconciled},
		StatusCancelled:       {StatusReconciled},
		StatusExpired:         {StatusReconciled},
	}
	return contains(allowed[current], next)
}

func contains(values []Status, value Status) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
