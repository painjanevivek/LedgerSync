package db

import (
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
)

func TestTransferCursorIsBoundToCanonicalFilters(t *testing.T) {
	filter := investigation.TransferFilter{
		Status: "posted", Query: "ABC-def",
		From: time.Date(2026, 8, 24, 10, 0, 0, 0, time.FixedZone("offset", 5*60*60+30*60)),
	}
	fingerprint := transferFilterFingerprint(filter)
	id := "00000000-0000-4000-8000-000000000001"
	at := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	cursor := encodeTransferCursor(at, id, fingerprint)
	decoded, err := decodeTransferCursor(cursor, transferFilterFingerprint(investigation.TransferFilter{Status: "posted", Query: "abc-DEF", From: filter.From.UTC()}))
	if err != nil || decoded.ID != id || !decoded.At.Equal(at) {
		t.Fatalf("decoded=%#v err=%v", decoded, err)
	}
	if _, err := decodeTransferCursor(cursor, transferFilterFingerprint(investigation.TransferFilter{Status: "rejected", Query: filter.Query, From: filter.From})); err == nil {
		t.Fatal("cursor was accepted after financial-status filter changed")
	}
	legacy := encodeInvestigationCursor(at, id)
	if _, err := decodeTransferCursor(legacy, transferFilterFingerprint(investigation.TransferFilter{})); err != nil {
		t.Fatalf("legacy unfiltered cursor was not preserved: %v", err)
	}
	if _, err := decodeTransferCursor(strings.Repeat("a", 769), fingerprint); err == nil {
		t.Fatal("oversized transfer cursor was accepted")
	}
}

func TestRelatedEvidenceRequiresSourceScopeAndUsesBoundedMetadataQueries(t *testing.T) {
	full := investigation.RelationshipAccess{Accounts: true, Transfers: true, Funding: true, Events: true, Reconciliation: true, Corrections: true}
	for _, sourceType := range []string{"account", "transfer", "funding", "event", "reconciliation_run", "reconciliation_mismatch", "correction"} {
		if !relationshipSourceAllowed(sourceType, full) {
			t.Fatalf("released source type %q was not authorized", sourceType)
		}
		query, args := relationshipQuery(investigation.RelationshipFilter{SourceType: sourceType, SourceID: "11111111-1111-4111-8111-111111111111", Limit: 20, Access: full}, "tenant", "actor")
		if !strings.Contains(query, "LIMIT $10") || len(args) != 10 || args[9] != 21 {
			t.Fatalf("source=%s query is not hard bounded: args=%#v", sourceType, args)
		}
		for _, forbidden := range []string{"amount_minor", "available_minor", "ledger_minor", "payload", "operator_note", "evidence_reference"} {
			if strings.Contains(query, forbidden) {
				t.Fatalf("source=%s relationship query copied forbidden field %q", sourceType, forbidden)
			}
		}
	}
	if relationshipSourceAllowed("transfer", investigation.RelationshipAccess{Accounts: true}) || relationshipSourceAllowed("unknown", full) {
		t.Fatal("relationship source was authorized without its own domain scope")
	}
}
