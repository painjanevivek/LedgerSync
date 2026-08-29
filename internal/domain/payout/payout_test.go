package payout

import (
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"testing"
	"time"
)

func TestPayoutRequiresINRExactFeeAndDualControl(t *testing.T) {
	amount, _ := money.New("INR", 10_000)
	fee, _ := money.New("INR", 125)
	p, err := New("payout-1", "tenant-1", "source-1", "beneficiary-1", "requester", amount, fee, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []Status{StatusReserved, StatusPendingApproval} {
		if err := p.Transition(state, "requester"); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Transition(StatusApproved, "requester"); err == nil {
		t.Fatal("requester approved payout")
	}
	if err := p.Transition(StatusApproved, "approver"); err != nil {
		t.Fatal(err)
	}
}
