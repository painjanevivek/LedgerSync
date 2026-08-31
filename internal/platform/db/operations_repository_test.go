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

func TestScanWebhookEndpointEvidenceDecodesPortableArrayProjection(t *testing.T) {
	item, err := scanWebhookEndpointEvidence(webhookScanFunc(func(destination ...any) error {
		*destination[0].(*string) = "70000000-0000-4000-8000-000000000071"
		*destination[1].(*string) = "Settlement partner"
		*destination[2].(*string) = "https://partner.example.test/hooks"
		*destination[3].(*string) = "active"
		*destination[4].(*[]byte) = []byte(`["transfer.posted","funding.posted"]`)
		*destination[5].(*string) = "delivered"
		*destination[6].(*string) = "2"
		*destination[7].(*string) = "0"
		*destination[11].(*time.Time) = time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(item.SubscribedEvents) != 2 || item.SubscribedEvents[1] != "funding.posted" {
		t.Fatalf("subscriptions=%#v", item.SubscribedEvents)
	}
}

func TestScanWebhookEndpointEvidenceRejectsMalformedArrayProjection(t *testing.T) {
	_, err := scanWebhookEndpointEvidence(webhookScanFunc(func(destination ...any) error {
		*destination[4].(*[]byte) = []byte(`not-json`)
		return nil
	}))
	if err == nil || err.Error() != "invalid persisted webhook subscriptions" {
		t.Fatalf("error=%v", err)
	}
}
