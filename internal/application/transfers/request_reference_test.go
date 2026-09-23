package transfers

import (
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

func TestSubmitValidatesAndNormalizesOptionalRequestReference(t *testing.T) {
	amount, _ := money.New("USD", 100)
	command := Command{TenantID: "tenant", ActorSubjectID: "actor", DebitAccountID: "a", CreditAccountID: "b", Amount: amount, IdempotencyKey: "request-reference-key", RequestReference: "11111111-1111-4111-8111-111111111111", OccurredAt: time.Now()}
	if err := validateCommand(normalize(command)); err != nil {
		t.Fatal(err)
	}
	command.RequestReference = "not-a-uuid"
	if err := validateCommand(normalize(command)); err == nil {
		t.Fatal("malformed request reference accepted")
	}
}
