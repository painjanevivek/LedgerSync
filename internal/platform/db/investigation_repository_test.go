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
