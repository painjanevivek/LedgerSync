package events

import (
	"math"
	"strconv"
	"testing"
)

func TestCausalMetadataAcceptsLegacyAndRejectsInvalidPairs(t *testing.T) {
	if value, err := decodeCausal(map[string]any{}, "origin", ""); err != nil || value != nil {
		t.Fatal("legacy metadata was not accepted")
	}
	for _, values := range []map[string]any{
		{"origin_node_id": "node"},
		{"origin_node_id": "node", "origin_logical_timestamp": "0"},
		{"origin_node_id": "node", "origin_logical_timestamp": "-1"},
		{"origin_node_id": "node", "origin_logical_timestamp": "9223372036854775808"},
		{"origin_node_id": "node", "origin_logical_timestamp": 1},
	} {
		if _, err := decodeCausal(values, "origin", ""); err == nil {
			t.Fatal("invalid causal metadata was accepted")
		}
	}
	value, err := decodeCausal(map[string]any{"origin_node_id": "node", "origin_logical_timestamp": strconv.FormatInt(math.MaxInt64, 10)}, "origin", "event")
	if err != nil || value.Timestamp != math.MaxInt64 {
		t.Fatal("exact logical timestamp was lost")
	}
}
