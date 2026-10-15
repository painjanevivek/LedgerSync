package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

type semanticPostingFixture struct {
	accountID string
	direction string
	amount    int64
	currency  string
}

func TestTransferJournalSemanticConstraintRejectsBalancedFalseShapes(t *testing.T) {
	cases := []struct {
		name            string
		postings        []semanticPostingFixture
		linkedJournalID string
		sourceID        string
		wantAllowed     bool
	}{
		{
			name: "canonical control", wantAllowed: true,
			postings: []semanticPostingFixture{{testSourceID, "debit", 1, "USD"}, {testDestinationID, "credit", 1, "USD"}},
		},
		{
			name: "balanced extra pair",
			postings: []semanticPostingFixture{
				{testSourceID, "debit", 1, "USD"}, {testDestinationID, "credit", 1, "USD"},
				{testSourceID, "debit", 1, "USD"}, {testDestinationID, "credit", 1, "USD"},
			},
		},
		{
			name:     "swapped direction",
			postings: []semanticPostingFixture{{testDestinationID, "debit", 1, "USD"}, {testSourceID, "credit", 1, "USD"}},
		},
		{
			name:     "wrong account",
			postings: []semanticPostingFixture{{testSourceID, "debit", 1, "USD"}, {testSourceID, "credit", 1, "USD"}},
		},
		{
			name:     "wrong amount",
			postings: []semanticPostingFixture{{testSourceID, "debit", 2, "USD"}, {testDestinationID, "credit", 2, "USD"}},
		},
		{
			name:     "unbalanced amount",
			postings: []semanticPostingFixture{{testSourceID, "debit", 1, "USD"}, {testDestinationID, "credit", 2, "USD"}},
		},
		{
			name:     "wrong currency",
			postings: []semanticPostingFixture{{testSourceID, "debit", 1, "EUR"}, {testDestinationID, "credit", 1, "EUR"}},
		},
		{name: "mutual command link missing", linkedJournalID: "00000000-0000-4000-8000-000000009999", postings: []semanticPostingFixture{{testSourceID, "debit", 1, "USD"}, {testDestinationID, "credit", 1, "USD"}}},
		{name: "forged source identity", sourceID: "00000000-0000-4000-8000-000000009998", postings: []semanticPostingFixture{{testSourceID, "debit", 1, "USD"}, {testDestinationID, "credit", 1, "USD"}}},
	}

	for index, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, database := requireTransferService(t, 10_000)
			err := exerciseTransferSemanticShape(t, database, index+1, testCase.postings, testCase.linkedJournalID, testCase.sourceID)
			if testCase.wantAllowed {
				if err != nil {
					t.Fatalf("canonical journal rejected: %v", err)
				}
				return
			}
			if state := sqlState(err); state != "23514" && state != "23503" {
				t.Fatalf("invalid semantic shape SQLSTATE=%s error=%v, want 23514/23503", state, err)
			}
		})
	}
}

func TestLedgerSemanticConstraintRejectsPostingMutationAndDeletion(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	result, err := service.Submit(ctx, transferCommand(t, "semantic-mutation-control-0001", "1.00"))
	if err != nil {
		t.Fatalf("submit mutation control: %v", err)
	}
	var journalID string
	if err = database.QueryRowContext(ctx, `SELECT journal_transaction_id::text FROM transfers WHERE id=$1`, result.Result.TransferID).Scan(&journalID); err != nil {
		t.Fatalf("read mutation control journal: %v", err)
	}

	for _, test := range []struct {
		name string
		sql  string
	}{
		{
			name: "change posting amount",
			sql:  `UPDATE ledger_postings SET amount_minor=amount_minor+1 WHERE journal_transaction_id=$1 AND direction='debit'`,
		},
		{
			name: "delete posting",
			sql:  `DELETE FROM ledger_postings WHERE journal_transaction_id=$1 AND direction='credit'`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, beginErr := database.BeginTx(ctx, nil)
			if beginErr != nil {
				t.Fatal(beginErr)
			}
			defer func() { _ = tx.Rollback() }()
			if _, execErr := tx.ExecContext(ctx, test.sql, journalID); execErr != nil {
				if sqlState(execErr) == "55000" {
					// The immutable-ledger trigger is the earlier defense. The semantic
					// trigger remains necessary if that implementation is ever replaced.
					return
				}
				t.Fatalf("stage posting mutation: %v", execErr)
			}
			_, execErr := tx.ExecContext(ctx, `SET CONSTRAINTS ALL IMMEDIATE`)
			if sqlState(execErr) != "23514" {
				t.Fatalf("posting mutation SQLSTATE=%s error=%v, want 23514", sqlState(execErr), execErr)
			}
		})
	}
}

func TestTransferJournalSemanticConstraintGeneratedShapeProperties(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	accounts := []string{testSourceID, testDestinationID}
	sequence := 100
	for _, debitAccount := range accounts {
		for _, creditAccount := range accounts {
			for _, amount := range []int64{1, 2} {
				for _, swappedDirections := range []bool{false, true} {
					sequence++
					debitDirection, creditDirection := "debit", "credit"
					if swappedDirections {
						debitDirection, creditDirection = creditDirection, debitDirection
					}
					postings := []semanticPostingFixture{
						{debitAccount, debitDirection, amount, "USD"},
						{creditAccount, creditDirection, amount, "USD"},
					}
					err := exerciseTransferSemanticShape(t, database, sequence, postings, "", "")
					effectiveDebitAccount, effectiveCreditAccount := debitAccount, creditAccount
					if swappedDirections {
						effectiveDebitAccount, effectiveCreditAccount = creditAccount, debitAccount
					}
					valid := effectiveDebitAccount == testSourceID && effectiveCreditAccount == testDestinationID && amount == 1
					if valid && err != nil {
						t.Fatalf("generated canonical shape rejected: %v", err)
					}
					if !valid && sqlState(err) != "23514" {
						t.Fatalf("generated invalid shape debit=%s credit=%s amount=%d swapped=%t SQLSTATE=%s error=%v", debitAccount, creditAccount, amount, swappedDirections, sqlState(err), err)
					}
				}
			}
		}
	}
}

func TestTransferCompensationMustExactlyInvertOriginal(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	original, err := service.Submit(context.Background(), transferCommand(t, "semantic-original-transfer-0001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}

	tx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	const compensationID = "00000000-0000-4000-8000-000000000951"
	const journalID = "00000000-0000-4000-8000-000000000952"
	now := time.Now().UTC()
	if _, err = tx.Exec(`
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,journal_transaction_id,created_at,completed_at,policy_version,compensation_of_transfer_id)
VALUES($1,$2,$3,$4,$5,2,'USD','posted',$6,$7,$7,1,$8)`, compensationID, testTenantID, testActorID, testDestinationID, testSourceID, journalID, now, original.Result.TransferID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at) VALUES($1,$2,$3,'transfer',$3,$4)`, journalID, testTenantID, compensationID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO ledger_postings(id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at) VALUES
('00000000-0000-4000-8000-000000000953',$1,$2,$3,'debit',2,'USD',$5),
('00000000-0000-4000-8000-000000000954',$1,$2,$4,'credit',2,'USD',$5)`, journalID, testTenantID, testDestinationID, testSourceID, now); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(`SET CONSTRAINTS ALL IMMEDIATE`)
	if sqlState(err) != "23514" {
		t.Fatalf("forged compensation SQLSTATE=%s error=%v, want 23514", sqlState(err), err)
	}
}

func TestFundingJournalMustUseExactClearingCustomerPair(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	const systemAccountID = "00000000-0000-4000-8000-000000000961"
	const fundingEventID = "00000000-0000-4000-8000-000000000962"
	const journalID = "00000000-0000-4000-8000-000000000963"
	now := time.Now().UTC()
	if _, err := database.ExecContext(ctx, `
INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference,account_kind,created_at,updated_at)
VALUES($1,$2,'USD','active','Semantic clearing','system','semantic-clearing','funding_clearing',$3,$3)`, systemAccountID, testTenantID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version,allow_negative) VALUES($1,0,0,0,true)`, systemAccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_opening_balances(account_id,opening_ledger_minor) VALUES($1,0)`, systemAccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO funding_events(id,tenant_id,requester_subject_id,approver_subject_id,destination_account_id,system_account_id,
 external_reference,evidence_reference,idempotency_key,request_fingerprint,amount_minor,currency,status,demo_policy,policy_version,
 correlation_id,requested_at,approved_at,updated_at)
VALUES($1,$2,'requester','approver',$3,$4,'semantic-funding','evidence://semantic','semantic-funding-request-0001',
 decode(repeat('00',32),'hex'),1,'USD','approved',false,1,'00000000-0000-4000-8000-000000000964',$5,$5,$5)`, fundingEventID, testTenantID, testDestinationID, systemAccountID, now); err != nil {
		t.Fatal(err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`INSERT INTO journal_transactions(id,tenant_id,funding_event_id,source_type,source_id,occurred_at) VALUES($1,$2,$3,'funding_event',$3,$4)`, journalID, testTenantID, fundingEventID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`
INSERT INTO ledger_postings(id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at) VALUES
('00000000-0000-4000-8000-000000000965',$1,$2,$3,'debit',1,'USD',$5),
('00000000-0000-4000-8000-000000000966',$1,$2,$4,'credit',1,'USD',$5)`, journalID, testTenantID, testDestinationID, systemAccountID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`UPDATE funding_events SET status='posted',journal_transaction_id=$2,posted_at=$3,updated_at=$3 WHERE id=$1`, fundingEventID, journalID, now); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(`SET CONSTRAINTS ALL IMMEDIATE`)
	if sqlState(err) != "23514" {
		t.Fatalf("forged funding pair SQLSTATE=%s error=%v, want 23514", sqlState(err), err)
	}
}

func exerciseTransferSemanticShape(t *testing.T, database *sql.DB, sequence int, postings []semanticPostingFixture, linkedJournalID, sourceID string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	transferID := fmt.Sprintf("00000000-0000-4000-8000-%012d", 9700+sequence*10)
	journalID := fmt.Sprintf("00000000-0000-4000-8000-%012d", 9701+sequence*10)
	if linkedJournalID == "" {
		linkedJournalID = journalID
	}
	if sourceID == "" {
		sourceID = transferID
	}
	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx, `
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,created_at,policy_version)
VALUES($1,$2,$3,$4,$5,1,'USD','pending',$6,1)`, transferID, testTenantID, testActorID, testSourceID, testDestinationID, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at) VALUES($1,$2,$3,'transfer',$4,$5)`, journalID, testTenantID, transferID, sourceID, now); err != nil {
		return err
	}
	for index, posting := range postings {
		postingID := fmt.Sprintf("00000000-0000-4000-8000-%012d", 9702+sequence*10+index)
		if _, err = tx.ExecContext(ctx, `
INSERT INTO ledger_postings(id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, postingID, journalID, testTenantID, posting.accountID, posting.direction, posting.amount, posting.currency, now); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE transfers SET status='posted',journal_transaction_id=$2,completed_at=$3 WHERE id=$1`, transferID, linkedJournalID, now); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `SET CONSTRAINTS ALL IMMEDIATE`)
	return err
}
