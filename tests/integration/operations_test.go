package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestEventEvidenceAuthorizationPaginationAndFirstClaimTruth(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	if _, err := transferService.Submit(context.Background(), transferCommand(t, "operations-event-evidence-key-01", "25.00")); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewOperationsRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	events, err := operations.NewEventService(repository)
	if err != nil {
		t.Fatal(err)
	}
	page, cursor, err := events.List(context.Background(), testTenantID, testActorID, operations.EventFilter{State: "pending", Limit: 1})
	if err != nil || len(page) != 1 || cursor == "" || page[0].State != "pending" {
		t.Fatalf("first page=%#v cursor=%q error=%v", page, cursor, err)
	}
	if _, _, err := events.List(context.Background(), testTenantID, testActorID, operations.EventFilter{Cursor: cursor, State: "published", Limit: 1}); err == nil {
		t.Fatal("cursor was accepted after its filter changed")
	}
	correlated, _, err := events.List(context.Background(), testTenantID, testActorID, operations.EventFilter{CorrelationID: "00000000-0000-0000-0000-000000000099", Limit: 25})
	if err != nil || len(correlated) != 2 {
		t.Fatalf("transfer correlation evidence=%#v error=%v", correlated, err)
	}
	notCorrelated, _, err := events.List(context.Background(), testTenantID, testActorID, operations.EventFilter{CorrelationID: "00000000-0000-0000-0000-000000000098", Limit: 25})
	if err != nil || len(notCorrelated) != 0 {
		t.Fatalf("unrelated correlation disclosed evidence: items=%#v error=%v", notCorrelated, err)
	}
	unauthorized, _, err := events.List(context.Background(), testTenantID, "unrelated-subject", operations.EventFilter{Limit: 25})
	if err != nil || len(unauthorized) != 0 {
		t.Fatalf("unauthorized list disclosed evidence: items=%#v error=%v", unauthorized, err)
	}
	if _, err := events.Get(context.Background(), testTenantID, "unrelated-subject", page[0].EventID); !errors.Is(err, db.ErrInvestigationNotFound) {
		t.Fatalf("unauthorized detail error=%v", err)
	}

	var latestAvailableAt time.Time
	if err := database.QueryRowContext(context.Background(), `SELECT max(available_at) FROM outbox_events WHERE tenant_id=$1`, testTenantID).Scan(&latestAvailableAt); err != nil {
		t.Fatal(err)
	}
	claimTime := latestAvailableAt.UTC().Add(time.Second)
	outboxRepository, err := db.NewOutboxRepository(database, func() time.Time { return claimTime })
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := outboxRepository.Claim(context.Background(), "operations-test-worker", 1, time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim=%#v error=%v", claimed, err)
	}
	claimedDetail, err := events.Get(context.Background(), testTenantID, testActorID, claimed[0].ID)
	if err != nil || claimedDetail.State != "pending" {
		t.Fatalf("healthy first claim mislabeled: detail=%#v error=%v", claimedDetail, err)
	}
	if err := outboxRepository.Reschedule(context.Background(), "operations-test-worker", claimed[0].ID, claimTime.Add(time.Minute), "publish_failed"); err != nil {
		t.Fatal(err)
	}
	retrying, _, err := events.List(context.Background(), testTenantID, testActorID, operations.EventFilter{State: "retrying", Limit: 25})
	if err != nil || len(retrying) != 1 || retrying[0].EventID != claimed[0].ID {
		t.Fatalf("rescheduled evidence=%#v error=%v", retrying, err)
	}
}

func TestEventEvidenceRedactsHostileCodesAndNeverReturnsPayloadOrEndpoint(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	result, err := transferService.Submit(context.Background(), transferCommand(t, "operations-redaction-key-001", "25.00"))
	if err != nil {
		t.Fatal(err)
	}
	var eventID string
	if err := database.QueryRow(`UPDATE outbox_events SET attempt_count=1,last_error_code='supersecret' WHERE tenant_id=$1 RETURNING id`, testTenantID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,response_class,sanitized_error_code,due_at,completed_at) VALUES('00000000-0000-0000-0000-000000000091',$1,$2,$3,'webhook','https://secret:token@example.invalid/hook',1,'dead','supersecret','token_abc123',now(),now())`, testTenantID, result.Result.TransferID, eventID); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewOperationsRepository(database)
	events, _ := operations.NewEventService(repository)
	detail, err := events.Get(context.Background(), testTenantID, testActorID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.LastErrorCode != "redacted" || len(detail.DeliveryAttempts) != 1 || detail.DeliveryAttempts[0].ResponseClass != "redacted" || detail.DeliveryAttempts[0].ErrorCode != "redacted" {
		t.Fatalf("hostile evidence survived redaction: %#v", detail)
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"supersecret", "token_abc123", "example.invalid", "endpoint_reference", "payload", "claim_owner"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("response exposed %q: %s", forbidden, encoded)
		}
	}
}

func TestDiagnosticFactsAreTenantScopedAndWorkerProgressIsDatabaseDerived(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	if _, err := transferService.Submit(context.Background(), transferCommand(t, "operations-diagnostics-key-01", "25.00")); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewOperationsRepository(database)
	facts, err := repository.Facts(context.Background(), testTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if facts.SchemaVersion != "000021_webhook_controls.up.sql" || facts.PendingOutboxCount != 2 || facts.DeadOutboxCount != 0 || facts.OldestPendingAt.IsZero() {
		t.Fatalf("unexpected database-derived facts: %#v", facts)
	}
}
