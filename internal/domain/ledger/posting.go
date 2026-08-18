// Package ledger defines append-only double-entry posting invariants.
package ledger

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var ErrUnbalanced = errors.New("journal postings are not balanced")

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

// Posting contains a positive amount. Direction, rather than a signed float or
// signed decimal, determines whether it is a debit or credit.
type Posting struct {
	ID         string
	JournalID  string
	AccountID  string
	Direction  Direction
	Amount     money.Money
	OccurredAt time.Time
}

func NewPosting(id, journalID, accountID string, direction Direction, amount money.Money, at time.Time) (Posting, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(journalID) == "" || strings.TrimSpace(accountID) == "" {
		return Posting{}, fmt.Errorf("posting id, journal id, and account id are required")
	}
	if direction != Debit && direction != Credit {
		return Posting{}, fmt.Errorf("posting direction is invalid")
	}
	if !amount.IsPositive() {
		return Posting{}, fmt.Errorf("%w: posting amount must be positive", money.ErrInvalidAmount)
	}
	return Posting{ID: id, JournalID: journalID, AccountID: accountID, Direction: direction, Amount: amount, OccurredAt: at.UTC()}, nil
}

// ValidateBalanced enforces equality of debit and credit totals per currency.
// It intentionally permits multi-currency journals only when each currency
// independently balances; transfer use cases remain single-currency in v1.
func ValidateBalanced(postings []Posting) error {
	if len(postings) < 2 {
		return ErrUnbalanced
	}
	debits := make(map[string]money.Money)
	credits := make(map[string]money.Money)
	for _, posting := range postings {
		if !posting.Amount.IsPositive() || (posting.Direction != Debit && posting.Direction != Credit) {
			return ErrUnbalanced
		}
		key := posting.Amount.Currency().Code
		totals := debits
		if posting.Direction == Credit {
			totals = credits
		}
		current, ok := totals[key]
		if !ok {
			current, _ = money.New(key, 0)
		}
		next, err := current.Add(posting.Amount)
		if err != nil {
			return fmt.Errorf("sum %s postings: %w", posting.Direction, err)
		}
		totals[key] = next
	}
	for currency, debitTotal := range debits {
		creditTotal, ok := credits[currency]
		if !ok || creditTotal.Minor() != debitTotal.Minor() {
			return ErrUnbalanced
		}
	}
	for currency := range credits {
		if _, ok := debits[currency]; !ok {
			return ErrUnbalanced
		}
	}
	return nil
}
