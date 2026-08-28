package db

import (
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
)

func TestEventCursorBindsEveryFilterAndLimit(t *testing.T) {
	filter := operations.EventFilter{EventType: "account.balance.changed.v1", State: "retrying", RelatedID: "account", CorrelationID: "correlation", From: time.Unix(1, 0), To: time.Unix(2, 0), Limit: 25}
	fingerprint := eventFilterFingerprint(filter)
	encoded := encodeEventCursor(eventCursor{At: time.Unix(2, 0), ID: "00000000-0000-0000-0000-000000000091", Fingerprint: fingerprint})
	if _, err := decodeEventCursor(encoded, fingerprint); err != nil {
		t.Fatal(err)
	}
	filter.State = "published"
	if _, err := decodeEventCursor(encoded, eventFilterFingerprint(filter)); err == nil {
		t.Fatal("cursor was accepted under a changed filter")
	}
}

func TestEvidenceCodeUsesSemanticAllowlist(t *testing.T) {
	if got := allowedEvidenceValue("timeout", "redacted", "", "timeout"); got != "timeout" {
		t.Fatalf("known code=%q", got)
	}
	for _, hostile := range []string{"supersecret", "postgres://user:secret@host/db", "token_abc123"} {
		if got := allowedEvidenceValue(hostile, "redacted", "", "timeout"); got != "redacted" {
			t.Fatalf("hostile value %q survived as %q", hostile, got)
		}
	}
}
