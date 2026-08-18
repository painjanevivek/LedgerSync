package unit_test

import (
	"errors"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestRetryableTransactionClassificationIsNarrow(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "serialization SQLSTATE", err: errors.New("ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)"), want: true},
		{name: "deadlock SQLSTATE", err: errors.New("ERROR: deadlock detected (SQLSTATE 40P01)"), want: true},
		{name: "insufficient funds", err: errors.New("insufficient funds"), want: false},
		{name: "authorization", err: errors.New("account access denied"), want: false},
		{name: "connection refused", err: errors.New("connection refused"), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := db.IsRetryableTransactionError(test.err); got != test.want {
				t.Fatalf("IsRetryableTransactionError(%v) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}
