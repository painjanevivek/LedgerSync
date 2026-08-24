package unit_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

type validationRepository struct {
	called bool
}

func (repository *validationRepository) Submit(context.Context, transfers.Command, [sha256.Size]byte) (transfers.Submission, error) {
	repository.called = true
	return transfers.Submission{}, nil
}

func TestZeroTransferIsRejectedBeforeTheFinancialRepository(t *testing.T) {
	repository := &validationRepository{}
	service, err := transfers.NewService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	zero, err := money.New("INR", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Submit(context.Background(), transfers.Command{
		TenantID: "tenant", ActorSubjectID: "operator", DebitAccountID: "source", CreditAccountID: "destination",
		Amount: zero, IdempotencyKey: "zero-transfer-key-0001",
	})
	if !errors.Is(err, transfers.ErrInvalidCommand) {
		t.Fatalf("error=%v, want invalid command", err)
	}
	if repository.called {
		t.Fatal("zero transfer reached the financial repository")
	}
}
