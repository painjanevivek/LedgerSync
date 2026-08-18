package unit_test

import (
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/ledger"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

func TestJournalRequiresBalancedPostings(t *testing.T) {
	amount, _ := money.New("USD", 250)
	debit, _ := ledger.NewPosting("p1", "j1", "source", ledger.Debit, amount, time.Now())
	credit, _ := ledger.NewPosting("p2", "j1", "destination", ledger.Credit, amount, time.Now())
	if err := ledger.ValidateBalanced([]ledger.Posting{debit, credit}); err != nil {
		t.Fatalf("balanced postings rejected: %v", err)
	}

	tooLarge, _ := money.New("USD", 251)
	unbalanced, _ := ledger.NewPosting("p3", "j1", "destination", ledger.Credit, tooLarge, time.Now())
	if err := ledger.ValidateBalanced([]ledger.Posting{debit, unbalanced}); !errors.Is(err, ledger.ErrUnbalanced) {
		t.Fatalf("error = %v, want unbalanced", err)
	}
}
