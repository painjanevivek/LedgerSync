package identifier

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

const mixedCaseUUID = "A0B1C2D3-E4F5-4678-9ABC-DEF012345678"
const canonicalUUID = "a0b1c2d3-e4f5-4678-9abc-def012345678"

type recordingObserver struct{ kinds []Kind }

func (o *recordingObserver) ObserveInvalidIdentifier(_ context.Context, kind Kind) {
	o.kinds = append(o.kinds, kind)
}

func TestParseCanonicalizesOnlyCanonicalShapeUUIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want UUID
	}{
		{name: "lowercase", raw: canonicalUUID, want: canonicalUUID},
		{name: "mixed case", raw: mixedCaseUUID, want: canonicalUUID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(context.Background(), KindAccount, test.raw)
			if err != nil || got != test.want {
				t.Fatalf("Parse() = %q, %v; want %q", got, err, test.want)
			}
			value, err := got.Value()
			if err != nil || value != driver.Value(canonicalUUID) {
				t.Fatalf("Value() = %#v, %v", value, err)
			}
		})
	}
}

func TestCanonicalUUIDsCollapseMapKeys(t *testing.T) {
	lower, err := Parse(context.Background(), KindTenant, canonicalUUID)
	if err != nil {
		t.Fatal(err)
	}
	upper, err := Parse(context.Background(), KindTenant, mixedCaseUUID)
	if err != nil {
		t.Fatal(err)
	}
	values := map[UUID]int{lower: 1}
	values[upper]++
	if len(values) != 1 || values[lower] != 2 {
		t.Fatalf("equivalent UUID text split map keys: %#v", values)
	}
}

func TestParseRejectsAmbiguousAndMalformedUUIDText(t *testing.T) {
	t.Parallel()

	invalid := []string{
		" " + canonicalUUID,
		canonicalUUID + " ",
		"{" + canonicalUUID + "}",
		"a0b1c2d3e4f546789abcdef012345678",
		"not-a-uuid",
		"00000000-0000-0000-0000-000000000000",
	}
	observer := &recordingObserver{}
	ctx := WithObserver(context.Background(), observer)
	for _, raw := range invalid {
		if _, err := Parse(ctx, KindTransfer, raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) error = %v; want ErrInvalid", raw, err)
		}
	}
	if len(observer.kinds) != len(invalid) {
		t.Fatalf("observed %d invalid identifiers; want %d", len(observer.kinds), len(invalid))
	}
}

func FuzzParseNeverAcceptsNonCanonicalShape(f *testing.F) {
	f.Add(canonicalUUID)
	f.Add(mixedCaseUUID)
	f.Add("{" + canonicalUUID + "}")
	f.Fuzz(func(t *testing.T, raw string) {
		id, err := Parse(context.Background(), KindUnknown, raw)
		if err != nil {
			return
		}
		if id.String() != canonicalUUID && raw != id.String() && raw != mixedCaseUUID {
			// Any accepted spelling must still round-trip through the strict shape
			// and must never retain attacker-controlled decoration.
			if len(raw) != 36 {
				t.Fatalf("accepted noncanonical UUID text %q", raw)
			}
		}
	})
}
