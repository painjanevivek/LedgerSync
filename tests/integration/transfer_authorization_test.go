package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTransferAuthorizationFailsClosedBeforeObjectDisclosure(t *testing.T) {
	for name, mutate := range map[string]func(*transferCommandFixture){
		"other tenant": func(fixture *transferCommandFixture) { fixture.tenantID = "00000000-0000-0000-0000-000000000099" },
		"other actor":  func(fixture *transferCommandFixture) { fixture.actorID = "other-operator" },
		"other source": func(fixture *transferCommandFixture) { fixture.sourceID = "00000000-0000-0000-0000-000000000099" },
	} {
		t.Run(name, func(t *testing.T) {
			service, database := requireTransferService(t, 10_000)
			fixture := transferCommandFixture{tenantID: testTenantID, actorID: testActorID, sourceID: testSourceID}
			mutate(&fixture)
			command := transferCommand(t, "authorization-denial-key-0001", "1.00")
			command.TenantID = identifier.UUID(fixture.tenantID)
			command.ActorSubjectID = fixture.actorID
			command.DebitAccountID = identifier.UUID(fixture.sourceID)
			_, err := service.Submit(context.Background(), command)
			if !errors.Is(err, db.ErrAccountNotFound) && !errors.Is(err, db.ErrNotAuthorized) {
				t.Fatalf("error=%v, want non-disclosing authorization denial", err)
			}
			if countRows(t, database, `SELECT count(*) FROM transfers`) != 0 || countRows(t, database, `SELECT count(*) FROM ledger_postings`) != 0 {
				t.Fatal("unauthorized request changed financial state")
			}
		})
	}
}

type transferCommandFixture struct {
	tenantID string
	actorID  string
	sourceID string
}
