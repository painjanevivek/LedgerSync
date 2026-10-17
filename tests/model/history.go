package model

import (
	"errors"
	"math/big"
)

type CompletedOperation struct {
	Intent             Intent
	Invoked, Responded int64
	Outcome            Outcome
}

// CheckHistory searches at most eight completed commands and 100,000 nodes.
// A real-time predecessor must appear first. Asynchronous delivery is excluded.
func CheckHistory(initial *Ledger, history []CompletedOperation) ([]int, error) {
	if initial == nil || len(history) == 0 || len(history) > 8 {
		return nil, errors.New("history requires 1-8 complete operations")
	}
	for _, event := range history {
		if event.Invoked < 1 || event.Responded < event.Invoked {
			return nil, errors.New("invalid operation interval")
		}
	}
	visited, exhausted := 0, false
	var search func(*Ledger, []int, uint16) ([]int, bool)
	search = func(state *Ledger, order []int, mask uint16) ([]int, bool) {
		visited++
		if visited > 100000 {
			exhausted = true
			return nil, false
		}
		if len(order) == len(history) {
			return append([]int(nil), order...), true
		}
		for index, event := range history {
			if mask&(1<<index) != 0 {
				continue
			}
			ready := true
			for prior, candidate := range history {
				if prior != index && mask&(1<<prior) == 0 && candidate.Responded < event.Invoked {
					ready = false
				}
			}
			if !ready {
				continue
			}
			next := cloneLedger(state)
			if VerifyOutcome(next.Move(event.Intent), event.Outcome) != nil {
				continue
			}
			if witness, ok := search(next, append(order, index), mask|(1<<index)); ok {
				return witness, true
			}
		}
		return nil, false
	}
	if order, ok := search(cloneLedger(initial), nil, 0); ok {
		return order, nil
	}
	if exhausted {
		return nil, errors.New("history search exhausted; no acceptance decision")
	}
	return nil, errors.New("no serial order satisfies observations and real-time precedence")
}

func cloneLedger(source *Ledger) *Ledger {
	clone := &Ledger{Accounts: map[string]Account{}, requests: map[requestKey]stored{}, Journals: source.Journals, total: new(big.Int).Set(source.total)}
	for id, account := range source.Accounts {
		clone.Accounts[id] = account
	}
	for key, result := range source.requests {
		clone.requests[key] = result
	}
	return clone
}

func VerifyOutcome(expected, actual Outcome) error {
	if expected.Status != actual.Status || expected.Replayed != actual.Replayed || expected.Status == "posted" && expected.From.Version != actual.From.Version {
		return errors.New("observed command outcome differs from independent prediction")
	}
	return nil
}

type LedgerObservation struct {
	Accounts                   map[string]Account
	JournalCount, PostingCount int
	Unbalanced                 bool
}

func VerifyObservation(expected *Ledger, actual LedgerObservation) error {
	if expected == nil || len(actual.Accounts) != len(expected.Accounts) || actual.JournalCount != expected.Journals || actual.PostingCount != 2*expected.Journals || actual.Unbalanced {
		return errors.New("observed journal or account inventory differs from independent prediction")
	}
	for id, account := range expected.Accounts {
		got, found := actual.Accounts[id]
		if !found || got.Minor != account.Minor || got.Version != account.Version {
			return errors.New("observed balance or version differs from independent prediction")
		}
	}
	return nil
}
