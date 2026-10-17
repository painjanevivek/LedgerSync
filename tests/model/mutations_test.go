package model

import "testing"

// Mutate the harness adapter's observations, never production financial code.
// These are positive controls for the same comparators used by DB histories.
func TestHarnessDetectsSeededDuplicateUnauthorizedAndUnbalancedMutations(t *testing.T) {
	ledger := fixture()
	ledger.Move(move("original", "100"))
	snapshot := func() LedgerObservation {
		accounts := map[string]Account{}
		for id, account := range ledger.Accounts {
			accounts[id] = account
		}
		return LedgerObservation{Accounts: accounts, JournalCount: 1, PostingCount: 2}
	}
	if err := VerifyObservation(ledger, snapshot()); err != nil {
		t.Fatal(err)
	}
	for _, mutant := range []struct {
		name string
		edit func(*LedgerObservation)
	}{{"duplicate-posting", func(o *LedgerObservation) { o.JournalCount++; o.PostingCount += 2 }}, {"unbalanced-entry", func(o *LedgerObservation) { o.Unbalanced = true }}, {"wrong-balance", func(o *LedgerObservation) { a := o.Accounts["a"]; a.Minor = "899"; o.Accounts["a"] = a }}} {
		t.Run(mutant.name, func(t *testing.T) {
			observation := snapshot()
			mutant.edit(&observation)
			if err := VerifyObservation(ledger, observation); err == nil {
				t.Fatal("seeded adapter defect escaped the harness")
			}
		})
	}
	if err := VerifyOutcome(Outcome{Status: "forbidden"}, Outcome{Status: "posted"}); err == nil {
		t.Fatal("missing authorization mutation escaped the harness")
	}
}
