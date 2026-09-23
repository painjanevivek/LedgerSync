package events

import "testing"

func TestNewLiveInvestigationSignalsRejectsUnsafeNamespace(t *testing.T) {
	for _, namespace := range []string{"", "room *", "room?"} {
		if _, err := NewLiveInvestigationSignals(nil, namespace); err == nil {
			t.Fatalf("expected invalid client/namespace %q to fail", namespace)
		}
	}
}

func TestRedisStreamCursorContract(t *testing.T) {
	for _, valid := range []string{"0-0", "123-0", "123-42"} {
		if !redisStreamCursor.MatchString(valid) {
			t.Fatalf("expected %q to be a valid cursor", valid)
		}
	}
	for _, invalid := range []string{"", "-", "0", "1-*", "(1-0"} {
		if redisStreamCursor.MatchString(invalid) {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
}
