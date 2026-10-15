package investigation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLiveRoomValidationAndFindingEligibility(t *testing.T) {
	ref := "11111111-1111-4111-8111-111111111111"
	if got, err := NormalizeRequestReference(strings.ToUpper(ref)); err != nil || got != ref {
		t.Fatalf("normalize=%q err=%v", got, err)
	}
	if _, err := NormalizeRequestReference("not-a-reference"); err == nil {
		t.Fatal("malformed request reference accepted")
	}
	if !FindingAllowed("confirmed_completed", "completed", ref) || FindingAllowed("confirmed_completed", "in_progress", "") {
		t.Fatal("completed finding eligibility is unsafe")
	}
	if !FindingAllowed("still_unresolved", "unavailable", "") || FindingAllowed("confirmed_rejected", "completed", ref) {
		t.Fatal("finding eligibility is unsafe")
	}
}

func TestNormalizeLiveSignalRejectsMalformedAndOversizedEnvelopes(t *testing.T) {
	room := "11111111-1111-4111-8111-111111111111"
	valid := LiveSignal{RoomID: room, Role: "owner", Sequence: 1, Kind: "offer", Payload: json.RawMessage(`{"type":"offer","sdp":"v=0"}`)}
	if _, err := NormalizeLiveSignal(valid); err != nil {
		t.Fatal(err)
	}
	valid.Sequence = 0
	if _, err := NormalizeLiveSignal(valid); err == nil {
		t.Fatal("out-of-order sequence accepted")
	}
	valid.Sequence = 1
	valid.Payload = json.RawMessage(`{"type":"answer","sdp":"v=0"}`)
	if _, err := NormalizeLiveSignal(valid); err == nil {
		t.Fatal("cross-kind SDP accepted")
	}
	valid.Kind = "ice"
	valid.Payload = json.RawMessage(`{"candidate":"` + strings.Repeat("x", MaxLiveSignalBytes) + `"}`)
	if _, err := NormalizeLiveSignal(valid); err == nil {
		t.Fatal("oversized signal accepted")
	}
}
