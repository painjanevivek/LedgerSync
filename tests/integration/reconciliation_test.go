package integration_test

import (
	"context"
	"testing"
)

func TestPostedTransferProjectionReconcilesWithImmutableLedger(t *testing.T) {
	service, database := requireTransferService(t, 10000)
	result, err := service.Submit(context.Background(), transferCommand(t, "reconciliation-transfer-key-01", "25.00"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Result.Status != "posted" {
		t.Fatalf("status = %q", result.Result.Status)
	}
	const reconciliation = `
SELECT p.account_id, p.ledger_minor,
       COALESCE(SUM(CASE WHEN l.direction = 'credit' THEN l.amount_minor ELSE -l.amount_minor END), 0) AS recomputed
FROM account_balance_projections p
LEFT JOIN ledger_postings l ON l.account_id = p.account_id
WHERE p.account_id IN ($1, $2)
GROUP BY p.account_id, p.ledger_minor
ORDER BY p.account_id`
	rows, err := database.Query(reconciliation, testSourceID, testDestinationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var accountID string
		var projection, recomputed int64
		if err := rows.Scan(&accountID, &projection, &recomputed); err != nil {
			t.Fatal(err)
		}
		// Fixtures start with an opening balance that predates this transfer, so
		// reconcile the delta from this journal rather than the fixture seed.
		if accountID == testSourceID && projection != 7500 {
			t.Fatalf("source projection = %d", projection)
		}
		if accountID == testDestinationID && projection != 4500 {
			t.Fatalf("destination projection = %d", projection)
		}
		if accountID == testSourceID && recomputed != -2500 {
			t.Fatalf("source ledger delta = %d", recomputed)
		}
		if accountID == testDestinationID && recomputed != 2500 {
			t.Fatalf("destination ledger delta = %d", recomputed)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
