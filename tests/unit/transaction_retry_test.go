package unit_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestLedgerSemanticViolationClassificationIsExactAndSafe(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		recognized bool
	}{
		{
			name:       "deferred semantic trigger",
			err:        fmt.Errorf("commit financial command: %w", &pgconn.PgError{Code: "23514", ConstraintName: "ledger_semantic_validation", Detail: "sensitive account and amount"}),
			recognized: true,
		},
		{
			name:       "source identity check",
			err:        &pgconn.PgError{Code: "23514", ConstraintName: "journal_source_matches_command_check"},
			recognized: true,
		},
		{name: "already sanitized", err: db.ErrLedgerSemanticViolation, recognized: true},
		{
			name:       "unrelated check constraint",
			err:        &pgconn.PgError{Code: "23514", ConstraintName: "account_status_check"},
			recognized: false,
		},
		{
			name:       "same name wrong SQLSTATE",
			err:        &pgconn.PgError{Code: "23503", ConstraintName: "ledger_semantic_validation"},
			recognized: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := db.IsLedgerSemanticViolation(test.err); got != test.recognized {
				t.Fatalf("IsLedgerSemanticViolation()=%t, want %t", got, test.recognized)
			}
		})
	}
}
