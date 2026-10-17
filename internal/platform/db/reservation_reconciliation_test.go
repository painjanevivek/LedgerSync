package db

import (
	"database/sql"
	"testing"
)

func TestReservationReconciliationUsesIndependentObligationSum(t *testing.T) {
	number := func(value string) sql.NullString { return sql.NullString{String: value, Valid: true} }
	matched, err := compareAccount("synthetic-account", "INR", number("700"), number("1000"), number("3"), number("1000"), "0", "300")
	if err != nil || len(matched) != 0 {
		t.Fatal("valid held balance mismatched", matched, err)
	}
	changed, err := compareAccount("synthetic-account", "INR", number("700"), number("1000"), number("3"), number("1000"), "0", "301")
	if err != nil || len(changed) != 1 || changed[0].ExpectedMinor == nil || *changed[0].ExpectedMinor != 699 {
		t.Fatal("obligation difference was not detected", err)
	}
	wide, err := compareAccount("synthetic-account", "INR", number("0"), number("0"), number("3"), number("0"), "0", "18446744073709551615")
	if err != nil || len(wide) != 1 || wide[0].ExpectedMinor != nil || wide[0].Details["expected_numeric"] != "-18446744073709551615" {
		t.Fatal("wide expected value was replaced with zero", err)
	}
}
