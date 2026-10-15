package integration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTransferRecoveryRejectedOutcomeIsFinal(t *testing.T) {
	service, database := requireTransferService(t, 100)
	repo, _ := db.NewTransferRepository(database, nil)
	ctx := context.Background()
	command := transferCommand(t, "recovery-rejected-key-001", "10.00")
	command.RequestReference = "50000000-0000-4000-8000-000000000099"
	if err := repo.PrepareRecovery(ctx, command); err != nil {
		t.Fatal(err)
	}
	first, err := service.Submit(ctx, command)
	if err != nil || first.Result.Status != "rejected" {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := service.Submit(ctx, command)
	if err != nil || !second.Replayed || second.Result.TransferID != first.Result.TransferID || second.Result.Status != "rejected" {
		t.Fatalf("replay=%#v err=%v", second, err)
	}
	item, err := repo.GetRecovery(ctx, testTenantID, testActorID, command.RequestReference)
	if err != nil || item.State != "rejected" {
		t.Fatalf("saved rejection=%#v err=%v", item, err)
	}
	if countRows(t, database, "SELECT count(*) FROM ledger_postings") != 0 {
		t.Fatal("rejected recovery posted money")
	}
}

func TestTransferRecoveryDurableScopeAndIdentity(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	repo, _ := db.NewTransferRepository(database, nil)
	ctx := context.Background()
	c := transferCommand(t, "recovery-original-key-01", "1.00")
	c.RequestReference = "50000000-0000-4000-8000-000000000001"
	if err := repo.PrepareRecovery(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := repo.PrepareRecovery(ctx, c); err != nil {
		t.Fatal(err)
	}
	if countRows(t, database, "SELECT count(*) FROM transfers") != 0 {
		t.Fatal("prepare moved money")
	}
	changed := c
	changed.Amount = transferCommand(t, "recovery-original-key-01", "2.00").Amount
	if err := repo.PrepareRecovery(ctx, changed); !errors.Is(err, transfers.ErrIdempotencyConflict) {
		t.Fatalf("changed intent=%v", err)
	}
	page, err := repo.ListRecovery(ctx, testTenantID, testActorID, "")
	if err != nil || len(page.Requests) != 1 || page.Requests[0].State != "unresolved" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	for _, scope := range [][2]string{{testTenantID, "another-actor"}, {"00000000-0000-4000-8000-000000000999", testActorID}} {
		page, err := repo.ListRecovery(ctx, scope[0], scope[1], "")
		if err != nil || len(page.Requests) != 0 {
			t.Fatalf("scope disclosed: %#v %v", page, err)
		}
		if _, err := repo.GetRecovery(ctx, scope[0], scope[1], c.RequestReference); !errors.Is(err, db.ErrInvestigationNotFound) {
			t.Fatalf("scope error=%v", err)
		}
	}
	first, err := service.Submit(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Submit(ctx, c)
	if err != nil || !again.Replayed || first.Result.TransferID != again.Result.TransferID {
		t.Fatalf("replay=%#v err=%v", again, err)
	}
	item, err := repo.GetRecovery(ctx, testTenantID, testActorID, c.RequestReference)
	if err != nil || item.State != "posted" || item.TransferID != first.Result.TransferID {
		t.Fatalf("outcome=%#v err=%v", item, err)
	}
	if _, err := database.ExecContext(ctx, "UPDATE transfer_recovery_intents SET amount_minor=999"); err == nil {
		t.Fatal("intent mutation permitted")
	}
	if _, err := database.ExecContext(ctx, "DELETE FROM transfer_recovery_intents"); err == nil {
		t.Fatal("intent deletion permitted")
	}
	if _, err := database.ExecContext(ctx, "UPDATE accounts SET status='frozen' WHERE id=$1", testSourceID); err != nil {
		t.Fatal(err)
	}
	item, err = repo.GetRecovery(ctx, testTenantID, testActorID, c.RequestReference)
	if err != nil || item.State != "posted" {
		t.Fatal("frozen account blocked outcome discovery", err)
	}
	if _, err := database.ExecContext(ctx, "DELETE FROM account_owners WHERE subject_id=$1", testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetRecovery(ctx, testTenantID, testActorID, c.RequestReference); !errors.Is(err, db.ErrInvestigationNotFound) {
		t.Fatalf("revoked account disclosed: %v", err)
	}
}

func TestTransferRecoveryBoundedPagination(t *testing.T) {
	_, database := requireTransferService(t, 10000)
	repo, _ := db.NewTransferRepository(database, nil)
	ctx := context.Background()
	for i := 1; i <= 27; i++ {
		c := transferCommand(t, fmt.Sprintf("recovery-page-key-%04d", i), "1.00")
		c.RequestReference = fmt.Sprintf("50000000-0000-4000-8000-%012d", i)
		if err := repo.PrepareRecovery(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.ListRecovery(ctx, testTenantID, testActorID, "")
	if err != nil || len(first.Requests) != 25 || first.NextCursor == "" {
		t.Fatalf("first=%#v %v", first, err)
	}
	second, err := repo.ListRecovery(ctx, testTenantID, testActorID, first.NextCursor)
	if err != nil || len(second.Requests) != 2 || second.NextCursor != "" {
		t.Fatalf("second=%#v %v", second, err)
	}
	seen := map[string]bool{}
	for _, item := range first.Requests {
		seen[item.RequestReference] = true
	}
	for _, item := range second.Requests {
		if seen[item.RequestReference] {
			t.Fatal("page overlap")
		}
	}
	if _, err := repo.ListRecovery(ctx, testTenantID, testActorID, "bad-cursor"); !errors.Is(err, transfers.ErrInvalidCommand) {
		t.Fatalf("bad cursor=%v", err)
	}
}
