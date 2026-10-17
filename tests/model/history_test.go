package model

import "testing"

func TestHistoryRespectsRealTimeAndCompetingFunds(t *testing.T) {
	initial := fixture()
	a, b := move("a", "700"), move("b", "600")
	first := fixture().Move(a)
	after := fixture()
	after.Move(a)
	second := after.Move(b)
	history := []CompletedOperation{{Intent: a, Invoked: 1, Responded: 4, Outcome: first}, {Intent: b, Invoked: 2, Responded: 5, Outcome: second}}
	if _, err := CheckHistory(initial, history); err != nil {
		t.Fatal(err)
	}
	history[1].Outcome = fixture().Move(b)
	if _, err := CheckHistory(initial, history); err == nil {
		t.Fatal("two overdrawn successful debits were accepted")
	}
	// Only b-before-a explains these outcomes, but their real-time order forbids it.
	bFirst := fixture()
	bOutcome := bFirst.Move(b)
	aOutcome := bFirst.Move(a)
	history = []CompletedOperation{{Intent: a, Invoked: 1, Responded: 2, Outcome: aOutcome}, {Intent: b, Invoked: 3, Responded: 4, Outcome: bOutcome}}
	if _, err := CheckHistory(initial, history); err == nil {
		t.Fatal("checker ignored real-time precedence")
	}
}

func TestHistoryRejectsInvalidOrUnboundedIntervals(t *testing.T) {
	for _, events := range [][]CompletedOperation{nil, make([]CompletedOperation, 9), {{Invoked: 5, Responded: 4}}} {
		if _, err := CheckHistory(fixture(), events); err == nil {
			t.Fatal("invalid history was accepted")
		}
	}
}
