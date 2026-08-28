package idempotency

import (
	"errors"
	"testing"
)

func TestOperationNamespaceAndLengthDelimitedFingerprint(t *testing.T) {
	create := Fingerprint("tenant", "actor", "accounts.create.v1", "ab", "c")
	ambiguousWithoutLengths := Fingerprint("tenant", "actor", "accounts.create.v1", "a", "bc")
	update := Fingerprint("tenant", "actor", "accounts.update.v1", "ab", "c")
	if create == ambiguousWithoutLengths || create == update {
		t.Fatal("canonical fingerprint did not preserve field or operation boundaries")
	}
}

func TestResolveReplayConflictAndInProgress(t *testing.T) {
	fingerprint := Fingerprint("intent")
	other := Fingerprint("changed-intent")
	if resolution, err := Resolve(&Existing{Fingerprint: fingerprint, State: StateCompleted}, fingerprint); err != nil || resolution != ResolutionReplay {
		t.Fatalf("replay resolution=%q error=%v", resolution, err)
	}
	if _, err := Resolve(&Existing{Fingerprint: fingerprint, State: StateCompleted}, other); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed intent error=%v, want conflict", err)
	}
	if _, err := Resolve(&Existing{Fingerprint: fingerprint, State: StateInProgress}, fingerprint); !errors.Is(err, ErrInProgress) {
		t.Fatalf("in-progress error=%v, want in progress", err)
	}
}

func FuzzFingerprintFieldBoundaries(f *testing.F) {
	f.Add("one", "two")
	f.Fuzz(func(t *testing.T, first, second string) {
		if first == second {
			return
		}
		if Fingerprint(first, second) == Fingerprint(second, first) {
			t.Fatal("field order collision")
		}
	})
}
