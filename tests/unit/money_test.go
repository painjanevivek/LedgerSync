package unit_test

import (
	"errors"
	"math"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

func TestParseUsesExactMinorUnits(t *testing.T) {
	amount, err := money.Parse("USD", "123.40")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := amount.Minor(), int64(12340); got != want {
		t.Fatalf("minor units = %d, want %d", got, want)
	}
	if got, want := amount.String(), "USD 123.40"; got != want {
		t.Fatalf("string = %q, want %q", got, want)
	}
}

func TestParseRejectsUnrepresentablePrecision(t *testing.T) {
	if _, err := money.Parse("USD", "1.001"); !errors.Is(err, money.ErrInvalidAmount) {
		t.Fatalf("error = %v, want invalid amount", err)
	}
	if _, err := money.Parse("JPY", "1.0"); !errors.Is(err, money.ErrInvalidAmount) {
		t.Fatalf("error = %v, want invalid amount", err)
	}
}

func TestSubtractCannotUnderflow(t *testing.T) {
	available, _ := money.New("INR", 100)
	requested, _ := money.New("INR", 101)
	if _, err := available.Subtract(requested); !errors.Is(err, money.ErrInsufficientAmount) {
		t.Fatalf("error = %v, want insufficient amount", err)
	}
}

func TestParsePreservesSigned64BitBoundaryWithoutRounding(t *testing.T) {
	amount, err := money.Parse("INR", "92233720368547758.07")
	if err != nil {
		t.Fatalf("parse maximum signed-64-bit amount: %v", err)
	}
	if amount.Minor() != math.MaxInt64 || amount.String() != "INR 92233720368547758.07" {
		t.Fatalf("maximum amount was not preserved exactly: minor=%d formatted=%q", amount.Minor(), amount.String())
	}
	if _, err := money.Parse("INR", "92233720368547758.08"); !errors.Is(err, money.ErrInvalidAmount) {
		t.Fatalf("overflow error=%v, want invalid amount", err)
	}
}

func TestParseMakesWhitespaceAndInvalidBoundariesExplicit(t *testing.T) {
	outerWhitespace, err := money.Parse(" inr ", " 1.00 ")
	if err != nil || outerWhitespace.Minor() != 100 {
		t.Fatalf("normalized boundary amount=%#v err=%v", outerWhitespace, err)
	}
	zero, err := money.Parse("INR", "0.00")
	if err != nil || !zero.IsZero() {
		t.Fatalf("domain zero amount=%#v err=%v", zero, err)
	}
	for _, input := range []string{"-1.00", "+1.00", "1e3", "1 .00", "1.001", "", ".01", "1."} {
		if _, err := money.Parse("INR", input); !errors.Is(err, money.ErrInvalidAmount) {
			t.Errorf("input %q error=%v, want invalid amount", input, err)
		}
	}
	if _, err := money.Parse("BTC", "1.00"); !errors.Is(err, money.ErrUnsupportedCurrency) {
		t.Fatalf("unsupported currency error=%v", err)
	}
	if _, err := money.New("INR", -1); !errors.Is(err, money.ErrInvalidAmount) {
		t.Fatalf("negative minor-unit error=%v", err)
	}
}
