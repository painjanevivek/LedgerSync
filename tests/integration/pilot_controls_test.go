package integration_test

import (
	"context"
	"errors"
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

func TestFinalFinancialAndEvidenceRowsRejectMutation(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	submission, err := service.Submit(context.Background(), transferCommand(t, "immutable-controls-0001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`UPDATE transfers SET rejection_code='tamper' WHERE id='` + submission.Result.TransferID + `'`,
		`UPDATE idempotency_requests SET response_status=500 WHERE transfer_id='` + submission.Result.TransferID + `'`,
		`DELETE FROM audit_events WHERE target_id='` + submission.Result.TransferID + `'`,
		`DELETE FROM journal_transactions WHERE transfer_id='` + submission.Result.TransferID + `'`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err == nil {
			t.Fatalf("mutation unexpectedly succeeded: %s", statement)
		}
	}
}
