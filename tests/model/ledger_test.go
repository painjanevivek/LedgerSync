package model

import (
	"reflect"
	"strconv"
	"testing"
)

func fixture() *Ledger { return New(map[string]Account{"a": {Minor: "1000"}, "b": {Minor: "100"}}) }
func move(key, amount string) Intent {
	return Intent{Tenant: "tenant", Actor: "operator", Operation: "transfer", Key: key, From: "a", To: "b", Currency: "USD", Minor: amount}
}
func TestOraclePreservesExactReplayConflictAndRejectedIntent(t *testing.T) {
	ledger := fixture()
	intent := move("first", "125")
	first := ledger.Move(intent)
	if first.Status != "posted" || ledger.Accounts["a"].Minor != "875" || ledger.Accounts["b"].Minor != "225" {
		t.Fatal("wrong independent movement")
	}
	intent.Minor = "126"
	if ledger.Move(intent).Status != "conflict" {
		t.Fatal("changed intent accepted")
	}
	intent.Minor = "125"
	replay := ledger.Move(intent)
	if !replay.Replayed || replay.From != first.From || ledger.Journals != 1 {
		t.Fatal("replay changed effect or stored observation")
	}
	denied := move("insufficient", "2000")
	if ledger.Move(denied).Status != "rejected" {
		t.Fatal("insufficient funds accepted")
	}
	ledger.AddZeroSystem("clearing")
	fund := move("fund", "3000")
	fund.Operation = "funding"
	fund.From = "clearing"
	fund.To = "a"
	ledger.Move(fund)
	if got := ledger.Move(denied); got.Status != "rejected" || !got.Replayed {
		t.Fatal("funding changed a durable rejected intent")
	}
	if err := ledger.Check(); err != nil {
		t.Fatal(err)
	}
}
func TestOracleUsesWideArithmeticBeforeCheckingStorageBounds(t *testing.T) {
	ledger := New(map[string]Account{"a": {Minor: "1000"}, "b": {Minor: "9223372036854775807"}})
	if ledger.Move(move("overflow", "1")).Status != "overflow" || ledger.Journals != 0 {
		t.Fatal("overflow created a journal")
	}
	for _, amount := range []string{"0", "-1", "01", "1.00", "9223372036854775808"} {
		if ledger.Move(move(amount, amount)).Status != "invalid" {
			t.Fatal("ambiguous or oversized amount accepted")
		}
	}
	if err := ledger.Check(); err != nil {
		t.Fatal(err)
	}
}
func TestSeededGenerationAndBoundedMinimization(t *testing.T) {
	if !reflect.DeepEqual(Generate(17, 64), Generate(17, 64)) || reflect.DeepEqual(Generate(17, 64), Generate(18, 64)) {
		t.Fatal("seed reproduction failed")
	}
	sequence := []Operation{{Kind: "noise"}, {Kind: "cause"}, {Kind: "noise"}, {Kind: "effect"}, {Kind: "noise"}}
	fails := func(operations []Operation) bool {
		cause := false
		for _, op := range operations {
			if op.Kind == "cause" {
				cause = true
			}
			if op.Kind == "effect" && cause {
				return true
			}
		}
		return false
	}
	minimal := Minimize(sequence, 20, fails)
	if len(minimal) != 2 || !fails(minimal) {
		t.Fatalf("counterexample not minimized: %+v", minimal)
	}
}
func FuzzIndependentConservation(f *testing.F) {
	f.Add(int64(17))
	f.Add(int64(20261015))
	f.Fuzz(func(t *testing.T, seed int64) {
		ledger := fixture()
		var prior []Intent
		for i, op := range Generate(seed, 64) {
			intent := move(strconv.Itoa(i), strconv.FormatInt(op.Minor, 10))
			if op.Reverse {
				intent.From, intent.To = intent.To, intent.From
			}
			if op.Kind == "fund" {
				ledger.AddZeroSystem("clearing")
				intent.From = "clearing"
				intent.Operation = "funding"
			}
			if (op.Kind == "replay" || op.Kind == "conflict") && len(prior) > 0 {
				intent = prior[op.Reference%len(prior)]
				if op.Kind == "conflict" {
					intent.Minor += "1"
				}
			}
			outcome := ledger.Move(intent)
			if !outcome.Replayed && (outcome.Status == "posted" || outcome.Status == "rejected") {
				prior = append(prior, intent)
			}
			if err := ledger.Check(); err != nil {
				t.Fatal(err)
			}
		}
	})
}
