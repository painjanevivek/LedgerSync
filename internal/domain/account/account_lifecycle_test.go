package account

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var lifecycleTime = time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)

func lifecycleAccount(t *testing.T) Account {
	t.Helper()
	result, err := NewConfigured("account-1", "tenant-1", "INR", Metadata{DisplayName: "व्यापार खाता", ExternalReference: "OPS-INR", Category: "operating"}, Owner{SubjectID: "operator", Permission: PermissionDebit}, lifecycleTime)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestLifecycleTransitionMatrix(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Account) error
		want Status
	}{
		{name: "active to frozen", run: func(value *Account) error { return value.Freeze(1, lifecycleTime.Add(time.Minute)) }, want: StatusFrozen},
		{name: "frozen to active", run: func(value *Account) error {
			if err := value.Freeze(1, lifecycleTime.Add(time.Minute)); err != nil {
				return err
			}
			return value.Reactivate(2, lifecycleTime.Add(2*time.Minute))
		}, want: StatusActive},
		{name: "active to closed", run: func(value *Account) error {
			return value.Close(1, FinancialState{Consistent: true}, lifecycleTime.Add(time.Minute))
		}, want: StatusClosed},
		{name: "frozen to closed", run: func(value *Account) error {
			if err := value.Freeze(1, lifecycleTime.Add(time.Minute)); err != nil {
				return err
			}
			return value.Close(2, FinancialState{Consistent: true}, lifecycleTime.Add(2*time.Minute))
		}, want: StatusClosed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := lifecycleAccount(t)
			if err := test.run(&value); err != nil {
				t.Fatal(err)
			}
			if value.Status != test.want {
				t.Fatalf("status=%s, want %s", value.Status, test.want)
			}
		})
	}
}

func TestClosedIsTerminalAndIdentityRemainsImmutable(t *testing.T) {
	value := lifecycleAccount(t)
	originalID, originalTenant, originalCurrency, originalCreated := value.ID, value.TenantID, value.Currency, value.CreatedAt
	if err := value.Close(1, FinancialState{Consistent: true}, lifecycleTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := value.UpdateMetadata(Metadata{DisplayName: "Changed", ExternalReference: "changed", Category: "reserve"}, 2, lifecycleTime.Add(2*time.Minute)); !errors.Is(err, ErrTerminalStatus) {
		t.Fatalf("closed metadata error=%v, want terminal", err)
	}
	if err := value.Reactivate(2, lifecycleTime.Add(2*time.Minute)); !errors.Is(err, ErrTerminalStatus) {
		t.Fatalf("closed transition error=%v, want terminal", err)
	}
	if value.ID != originalID || value.TenantID != originalTenant || value.Currency != originalCurrency || value.CreatedAt != originalCreated {
		t.Fatal("immutable identity changed during lifecycle mutation")
	}
}

func TestCloseRequiresConsistentExactZeroFinancialState(t *testing.T) {
	for _, test := range []struct {
		name  string
		state FinancialState
		want  error
	}{
		{name: "unavailable", state: FinancialState{}, want: ErrFinancialStateUnavailable},
		{name: "available non-zero", state: FinancialState{Consistent: true, AvailableMinor: 1}, want: ErrNonZeroBalance},
		{name: "ledger non-zero", state: FinancialState{Consistent: true, LedgerMinor: 1}, want: ErrNonZeroBalance},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := lifecycleAccount(t)
			if err := value.Close(1, test.state, lifecycleTime.Add(time.Minute)); !errors.Is(err, test.want) {
				t.Fatalf("error=%v, want %v", err, test.want)
			}
		})
	}
}

func TestMetadataValidationPreservesUnicodeAndNormalizesReference(t *testing.T) {
	metadata, err := NormalizeMetadata(Metadata{DisplayName: "  Marketing — भारत  ", ExternalReference: " MKTG.INDIA_01 ", Category: " Expenses "})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DisplayName != "Marketing — भारत" || metadata.ExternalReference != "mktg.india_01" || metadata.Category != "expenses" {
		t.Fatalf("unexpected normalized metadata: %#v", metadata)
	}
	for _, invalid := range []Metadata{
		{DisplayName: "", ExternalReference: "valid-ref", Category: "operating"},
		{DisplayName: "name\nline", ExternalReference: "valid-ref", Category: "operating"},
		{DisplayName: strings.Repeat("x", MaxDisplayNameRunes+1), ExternalReference: "valid-ref", Category: "operating"},
		{DisplayName: "name", ExternalReference: "../unsafe", Category: "operating"},
		{DisplayName: "name", ExternalReference: "valid-ref", Category: "other"},
	} {
		if _, err := NormalizeMetadata(invalid); !errors.Is(err, ErrInvalidMetadata) {
			t.Fatalf("metadata=%#v error=%v, want invalid metadata", invalid, err)
		}
	}
}

func FuzzNormalizeMetadataStable(f *testing.F) {
	f.Add("Reserve भारत", "RESERVE-01", "reserve")
	f.Add("\x00", "valid-ref", "operating")
	f.Fuzz(func(t *testing.T, name, reference, category string) {
		first, err := NormalizeMetadata(Metadata{DisplayName: name, ExternalReference: reference, Category: category})
		if err != nil {
			return
		}
		second, err := NormalizeMetadata(first)
		if err != nil || first != second {
			t.Fatalf("accepted metadata is not stable: first=%#v second=%#v error=%v", first, second, err)
		}
	})
}
