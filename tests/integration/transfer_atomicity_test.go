package integration_test

import (
	"context"
	"testing"
)

func TestAcceptedTransferCommitsCompleteFinancialEvidenceTogether(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	submission, err := service.Submit(context.Background(), transferCommand(t, "complete-atomic-transfer-0001", "25.00"))
	if err != nil {
		t.Fatal(err)
	}
	transferID := submission.Result.TransferID

	checks := []struct {
		name  string
		query string
		want  int
	}{
		{"transfer", `SELECT count(*) FROM transfers WHERE id=$1 AND status='posted'`, 1},
		{"journal", `SELECT count(*) FROM journal_transactions WHERE transfer_id=$1`, 1},
		{"debit", `SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.transfer_id=$1 AND p.direction='debit'`, 1},
		{"credit", `SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.transfer_id=$1 AND p.direction='credit'`, 1},
		{"audit", `SELECT count(*) FROM audit_events WHERE target_id=$1 AND event_type='transfer.posted' AND outcome='succeeded'`, 1},
		{"idempotency outcome", `SELECT count(*) FROM idempotency_requests WHERE transfer_id=$1 AND state='completed' AND response_status=201`, 1},
		{"outbox events", `SELECT count(*) FROM outbox_events WHERE transfer_id=$1`, 2},
		{"velocity event", `SELECT count(*) FROM transfer_velocity_events WHERE transfer_id=$1`, 1},
	}
	for _, check := range checks {
		if got := countRows(t, database, check.query, transferID); got != check.want {
			t.Errorf("%s rows=%d, want %d", check.name, got, check.want)
		}
	}

	var sourceMinor, destinationMinor, sourceVersion, destinationVersion int64
	if err := database.QueryRow(`
SELECT
  MAX(available_minor) FILTER (WHERE account_id=$1),
  MAX(available_minor) FILTER (WHERE account_id=$2),
  MAX(balance_version) FILTER (WHERE account_id=$1),
  MAX(balance_version) FILTER (WHERE account_id=$2)
FROM account_balance_projections WHERE account_id IN ($1,$2)`, testSourceID, testDestinationID).
		Scan(&sourceMinor, &destinationMinor, &sourceVersion, &destinationVersion); err != nil {
		t.Fatal(err)
	}
	if sourceMinor != 7_500 || destinationMinor != 4_500 || sourceVersion != 1 || destinationVersion != 1 {
		t.Fatalf("balances source=%d@%d destination=%d@%d", sourceMinor, sourceVersion, destinationMinor, destinationVersion)
	}
	if submission.Result.MinimumBalanceVersions[testSourceID] != 1 || submission.Result.MinimumBalanceVersions[testDestinationID] != 1 {
		t.Fatalf("response versions=%v", submission.Result.MinimumBalanceVersions)
	}
}

func TestTransferRollsBackEverySideEffectWhenPostingFailsMidTransaction(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	if _, err := database.Exec(`
CREATE OR REPLACE FUNCTION integration_fail_credit_posting() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.direction = 'credit' THEN
    RAISE EXCEPTION 'injected credit-posting failure';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER integration_fail_credit_posting
BEFORE INSERT ON ledger_postings
FOR EACH ROW EXECUTE FUNCTION integration_fail_credit_posting()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DROP TRIGGER IF EXISTS integration_fail_credit_posting ON ledger_postings`)
		_, _ = database.Exec(`DROP FUNCTION IF EXISTS integration_fail_credit_posting()`)
	})

	if _, err := service.Submit(context.Background(), transferCommand(t, "atomic-rollback-transfer-0001", "25.00")); err == nil {
		t.Fatal("injected posting failure was reported as success")
	}
	for table, query := range map[string]string{
		"transfers":        `SELECT count(*) FROM transfers`,
		"journals":         `SELECT count(*) FROM journal_transactions`,
		"postings":         `SELECT count(*) FROM ledger_postings`,
		"audit events":     `SELECT count(*) FROM audit_events`,
		"idempotency rows": `SELECT count(*) FROM idempotency_requests`,
		"outbox events":    `SELECT count(*) FROM outbox_events`,
		"velocity events":  `SELECT count(*) FROM transfer_velocity_events`,
	} {
		if got := countRows(t, database, query); got != 0 {
			t.Errorf("%s survived rollback: %d", table, got)
		}
	}

	var sourceMinor, destinationMinor, sourceVersion, destinationVersion int64
	if err := database.QueryRow(`
SELECT
  MAX(available_minor) FILTER (WHERE account_id=$1),
  MAX(available_minor) FILTER (WHERE account_id=$2),
  MAX(balance_version) FILTER (WHERE account_id=$1),
  MAX(balance_version) FILTER (WHERE account_id=$2)
FROM account_balance_projections WHERE account_id IN ($1,$2)`, testSourceID, testDestinationID).
		Scan(&sourceMinor, &destinationMinor, &sourceVersion, &destinationVersion); err != nil {
		t.Fatal(err)
	}
	if sourceMinor != 10_000 || destinationMinor != 2_000 || sourceVersion != 0 || destinationVersion != 0 {
		t.Fatalf("rollback changed balances source=%d@%d destination=%d@%d", sourceMinor, sourceVersion, destinationMinor, destinationVersion)
	}
}
