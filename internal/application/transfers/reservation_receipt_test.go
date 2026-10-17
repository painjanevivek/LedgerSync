package transfers

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestReservationBalanceReceiptRoundTrip(t *testing.T) {
	source := `{"transfer_id":"synthetic-transfer","status":"posted","currency":"INR","amount_minor":"100","occurred_at":"2026-10-16T00:00:00Z","minimum_balance_versions":{"source":"4"},"balances":{"source":{"account_id":"source","currency":"INR","posted_minor":"1000","available_minor":"700","reserved_minor":"300","version":"4","as_of":"2026-10-16T00:00:00Z"}}}`
	var result Result
	if err := json.Unmarshal([]byte(source), &result); err != nil {
		t.Fatal(err)
	}
	if result.Balances["source"].PostedMinor != 1000 || result.Balances["source"].AvailableMinor != "700" || result.Balances["source"].ReservedMinor != "300" {
		t.Fatal("receipt meanings collapsed")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var before, after any
	_ = json.Unmarshal([]byte(source), &before)
	_ = json.Unmarshal(encoded, &after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("exact reserved receipt changed on replay")
	}
	invalid := result
	balance := invalid.Balances["source"]
	balance.ReservedMinor = "301"
	invalid.Balances["source"] = balance
	encoded, err = json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(encoded, &result) == nil {
		t.Fatal("invalid reserved equation accepted")
	}
}
func TestHistoricalBalanceReceiptOmitsNewReservationFields(t *testing.T) {
	original := Result{TransferID: "historical", Status: "posted", Currency: "INR", AmountMinor: 1, MinimumBalanceVersions: map[string]int64{}, Balances: map[string]Balance{"source": {AccountID: "source", Currency: "INR", PostedMinor: 1000, Version: 1, AsOf: "2026-10-16T00:00:00Z"}}}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var restored Result
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(restored)
	if err != nil || string(raw) != string(again) {
		t.Fatal("historical receipt changed", err)
	}
}
