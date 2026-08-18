package unit_test

import (
	"testing"
	"unicode/utf8"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

// FuzzParseExactMoney protects the public parsing boundary. Any input may be
// rejected, but no input may panic or produce a negative exact value.
func FuzzParseExactMoney(f *testing.F) {
	for _, seed := range []string{"0", "1", "12.50", "999999999999999999", "1.001", "1e3", "-1", "१२"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			t.Skip()
		}
		amount, err := money.Parse("USD", input)
		if err == nil && amount.Minor() < 0 {
			t.Fatalf("accepted negative minor amount %d for %q", amount.Minor(), input)
		}
	})
}
