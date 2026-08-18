package unit_test

import (
	"errors"
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
