package unit_test

import (
	"errors"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

func TestIdempotencyReplayAndMismatch(t *testing.T) {
	amount, _ := money.Parse("USD", "1.20")
	request := transfers.IdempotencyRequest{
		TenantID: "tenant", ActorSubjectID: "actor", Operation: "transfers.create.v1", Key: "0123456789abcdef",
		DebitAccountID: "source", CreditAccountID: "destination", Amount: amount,
	}
	fingerprint, err := transfers.Fingerprint(request)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	resolution, err := transfers.ResolveExisting(&transfers.ExistingRequest{Fingerprint: fingerprint, State: transfers.StateCompleted}, fingerprint)
	if err != nil || resolution != transfers.ResolutionReplay {
		t.Fatalf("resolution = %q, error = %v", resolution, err)
	}

	otherAmount, _ := money.Parse("USD", "1.21")
	request.Amount = otherAmount
	mismatch, _ := transfers.Fingerprint(request)
	_, err = transfers.ResolveExisting(&transfers.ExistingRequest{Fingerprint: fingerprint, State: transfers.StateCompleted}, mismatch)
	if !errors.Is(err, transfers.ErrIdempotencyConflict) {
		t.Fatalf("error = %v, want mismatch conflict", err)
	}
}
