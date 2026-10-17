package model

import "testing"

func TestIndependentFixtureCurrencyDoesNotPermitCrossCurrencyMovement(t *testing.T) {
	for _, currency := range []string{"USD", "INR"} {
		ledger := NewForCurrency(map[string]Account{"a": {Minor: "100"}, "b": {Minor: "0"}}, currency)
		other := "USD"
		if currency == other {
			other = "INR"
		}
		intent := Intent{Tenant: "tenant", Actor: "operator", Operation: "transfer", Key: "currency-negative", From: "a", To: "b", Currency: other, Minor: "1"}
		if ledger.Move(intent).Status != "invalid" || ledger.Journals != 0 {
			t.Fatal("cross-currency movement accepted")
		}
		intent.Currency, intent.Key = currency, "currency-positive"
		if ledger.Move(intent).Status != "posted" || ledger.Journals != 1 {
			t.Fatal("selected fixture currency rejected")
		}
		if err := ledger.Check(); err != nil {
			t.Fatal(err)
		}
	}
}
