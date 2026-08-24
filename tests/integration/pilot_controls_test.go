package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestRateLimitIsSharedByTenantPrincipalAndRoute(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	limiter, err := db.NewRateLimitRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := limiter.Consume(context.Background(), testTenantID, testActorID, "transfers:create", 1, time.Minute)
	if err != nil || !first.Allowed {
		t.Fatalf("first decision=%#v err=%v", first, err)
	}
	second, err := limiter.Consume(context.Background(), testTenantID, testActorID, "transfers:create", 1, time.Minute)
	if err != nil || second.Allowed || second.RetryAfter <= 0 {
		t.Fatalf("second decision=%#v err=%v", second, err)
	}
}

func TestRateLimitRejectsSubsecondWindows(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	limiter, err := db.NewRateLimitRepository(database, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limiter.Consume(context.Background(), testTenantID, testActorID, "transfers:create", 1, 500*time.Millisecond); err == nil {
		t.Fatal("subsecond fixed window was accepted")
	}
}

func TestDestinationRequiresExplicitActorRelationshipAndDenialIsAudited(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	if _, err := database.Exec(`DELETE FROM account_credit_permissions WHERE account_id=$1`, testDestinationID); err != nil {
		t.Fatal(err)
	}
	_, err := service.Submit(context.Background(), transferCommand(t, "destination-policy-0001", "1.00"))
	if !errors.Is(err, db.ErrDestinationNotAuthorized) {
		t.Fatalf("error=%v", err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM audit_events WHERE tenant_id=$1 AND event_type='transfer.policy_denied' AND sanitized_metadata->>'denial_code'='destination_not_authorized'`, testTenantID); got != 1 {
		t.Fatalf("denial audits=%d", got)
	}
	if got := countRows(t, database, `SELECT count(*) FROM transfers`); got != 0 {
		t.Fatalf("unauthorized transfers=%d", got)
	}
}

func TestConcurrentTransfersCannotBypassRollingActorLimit(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	if _, err := database.Exec(`UPDATE tenant_transfer_policies SET maximum_transfer_minor=100,actor_rolling_24h_minor=150,source_account_rolling_24h_minor=1000,tenant_rolling_24h_minor=1000 WHERE tenant_id=$1`, testTenantID); err != nil {
		t.Fatal(err)
	}
	commands := []string{"velocity-concurrency-0001", "velocity-concurrency-0002"}
	errs := make(chan error, len(commands))
	var wait sync.WaitGroup
	for _, key := range commands {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.Submit(context.Background(), transferCommand(t, key, "1.00"))
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	succeeded, denied := 0, 0
	for err := range errs {
		if err == nil {
			succeeded++
		} else if errors.Is(err, db.ErrActorVelocityExceeded) {
			denied++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 || denied != 1 {
		t.Fatalf("succeeded=%d denied=%d", succeeded, denied)
	}
	if got := countRows(t, database, `SELECT count(*) FROM transfers WHERE status='posted'`); got != 1 {
		t.Fatalf("posted=%d", got)
	}
}

func TestHotTenantSequenceAvoidsExhaustedSerializationConflicts(t *testing.T) {
	service, database := requireTransferService(t, 1_000_000)
	const submissions = 50
	errs := make(chan error, submissions)
	var wait sync.WaitGroup
	for index := 0; index < submissions; index++ {
		key := fmt.Sprintf("hot-tenant-capacity-%04d", index)
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			command := transferCommand(t, key, "1.00")
			if index%2 == 0 {
				command.TenantID = strings.ToUpper(command.TenantID)
			}
			_, err := service.Submit(context.Background(), command)
			errs <- err
		}(index)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("hot-tenant submission failed: %v", err)
		}
	}
	if got := countRows(t, database, `SELECT count(*) FROM transfers WHERE status='posted'`); got != submissions {
		t.Fatalf("posted transfers=%d, want %d", got, submissions)
	}
	if got := countRows(t, database, `SELECT count(*) FROM ledger_postings`); got != submissions*2 {
		t.Fatalf("ledger postings=%d, want %d", got, submissions*2)
	}
	if got := countRows(t, database, `SELECT count(*) FROM transfer_velocity_events`); got != submissions {
		t.Fatalf("velocity events=%d, want %d", got, submissions)
	}
}

func TestVelocityStatePrunesExpiredMovementAndDoesNotDoubleCountReplay(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	oldTransferID := "00000000-0000-0000-0000-000000000701"
	oldJournalID := "00000000-0000-0000-0000-000000000702"
	oldOccurredAt := time.Date(2026, 8, 17, 9, 14, 59, 0, time.UTC)
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`
INSERT INTO transfers (
  id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,
  amount_minor,currency,status,created_at
) VALUES ($1,$2,$3,$4,$5,100,'USD','pending',$6)`, oldTransferID, testTenantID,
		testActorID, testSourceID, testDestinationID, oldOccurredAt); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO journal_transactions (id,tenant_id,transfer_id,occurred_at) VALUES ($1,$2,$3,$4)`, oldJournalID, testTenantID, oldTransferID, oldOccurredAt); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO ledger_postings (id,journal_transaction_id,account_id,direction,amount_minor,currency,occurred_at) VALUES
  ('00000000-0000-0000-0000-000000000703',$1,$2,'debit',100,'USD',$4),
  ('00000000-0000-0000-0000-000000000704',$1,$3,'credit',100,'USD',$4)`, oldJournalID, testSourceID, testDestinationID, oldOccurredAt); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`UPDATE transfers SET status='posted',journal_transaction_id=$2,completed_at=$3 WHERE id=$1`, oldTransferID, oldJournalID, oldOccurredAt); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO transfer_velocity_events (
  transfer_id,tenant_id,actor_subject_id,source_account_id,
  amount_minor,occurred_at,expires_at
) VALUES ($1,$2,$3,$4,100,$5::timestamptz,$5::timestamptz + INTERVAL '24 hours')`, oldTransferID, testTenantID,
		testActorID, testSourceID, oldOccurredAt); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO transfer_velocity_totals (tenant_id,dimension_type,dimension_reference,total_minor) VALUES
  ($1::uuid,'tenant',$1::uuid::text,100),($1::uuid,'actor',$2,100),($1::uuid,'source',$3,100)`, testTenantID, testActorID, testSourceID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	command := transferCommand(t, "velocity-expiry-replay-0001", "1.00")
	first, err := service.Submit(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Submit(context.Background(), command)
	if err != nil || !replay.Replayed || replay.Result.TransferID != first.Result.TransferID {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM transfer_velocity_events`); got != 1 {
		t.Fatalf("active velocity events=%d, want 1", got)
	}
	var tenantTotal, actorTotal, sourceTotal int64
	if err := database.QueryRow(`
SELECT
  MAX(total_minor) FILTER (WHERE dimension_type='tenant'),
  MAX(total_minor) FILTER (WHERE dimension_type='actor'),
  MAX(total_minor) FILTER (WHERE dimension_type='source')
FROM transfer_velocity_totals WHERE tenant_id=$1`, testTenantID).Scan(&tenantTotal, &actorTotal, &sourceTotal); err != nil {
		t.Fatal(err)
	}
	if tenantTotal != 100 || actorTotal != 100 || sourceTotal != 100 {
		t.Fatalf("velocity totals tenant=%d actor=%d source=%d, want 100 each", tenantTotal, actorTotal, sourceTotal)
	}
}

func TestFinalFinancialAndEvidenceRowsRejectMutation(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	submission, err := service.Submit(context.Background(), transferCommand(t, "immutable-controls-0001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`UPDATE transfers SET rejection_code='tamper' WHERE id='` + submission.Result.TransferID + `'`,
		`DELETE FROM transfers WHERE id='` + submission.Result.TransferID + `'`,
		`UPDATE idempotency_requests SET response_status=500 WHERE transfer_id='` + submission.Result.TransferID + `'`,
		`DELETE FROM audit_events WHERE target_id='` + submission.Result.TransferID + `'`,
		`DELETE FROM journal_transactions WHERE transfer_id='` + submission.Result.TransferID + `'`,
		`UPDATE ledger_postings SET amount_minor=1 WHERE journal_transaction_id=(SELECT id FROM journal_transactions WHERE transfer_id='` + submission.Result.TransferID + `')`,
		`DELETE FROM ledger_postings WHERE journal_transaction_id=(SELECT id FROM journal_transactions WHERE transfer_id='` + submission.Result.TransferID + `')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err == nil {
			t.Fatalf("mutation unexpectedly succeeded: %s", statement)
		}
	}
}
