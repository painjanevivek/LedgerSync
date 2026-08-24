package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestAccountReadsDenyUnownedAccountsWithoutDisclosure(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	balances, err := db.NewBalanceRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := balances.ReadCurrent(ctx, testTenantID, "different-subject", testSourceID); !errors.Is(err, db.ErrBalanceNotAuthorized) {
		t.Fatalf("expected safe authorization denial, got %v", err)
	}
	if err := balances.Authorize(ctx, testTenantID, "different-subject", testSourceID); !errors.Is(err, db.ErrBalanceNotAuthorized) {
		t.Fatalf("expected safe cache authorization denial, got %v", err)
	}
	if err := balances.Authorize(ctx, testTenantID, testActorID, testSourceID); err != nil {
		t.Fatalf("authorized cache read was denied: %v", err)
	}
	historyRepo, err := db.NewTransactionHistoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := historyRepo.ListAccountHistory(ctx, testTenantID, "different-subject", testSourceID, "", 10); !errors.Is(err, transactions.ErrHistoryNotFound) {
		t.Fatalf("expected safe history denial, got %v", err)
	}
}

func TestAccountDirectoryPaginatesTenThousandAuthorizedAccountsWithStableIndexes(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	tenantID := "00000000-0000-0000-0000-000000009001"
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants(id,external_reference)VALUES($1,'scale-tenant')`, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
WITH generated AS (SELECT g,md5('scale-account-'||g)::uuid AS id FROM generate_series(1,10000) g)
INSERT INTO accounts(id,tenant_id,currency,status,created_at,display_name,category,external_reference)
SELECT id,$1,'USD',CASE WHEN g%10=0 THEN 'frozen' ELSE 'active' END,'2026-01-01T00:00:00Z'::timestamptz+(g||' milliseconds')::interval,'Scale account '||lpad(g::text,5,'0'),CASE WHEN g%2=0 THEN 'operating' ELSE 'customer_funds' END,'scale-'||lpad(g::text,5,'0') FROM generated`, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_owners(tenant_id,account_id,subject_id,permission) SELECT $1,id,'scale-operator','read' FROM accounts WHERE tenant_id=$1`, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version,updated_at) SELECT id,100,100,1,'2026-01-01T00:00:00Z' FROM accounts WHERE tenant_id=$1`, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `ANALYZE accounts; ANALYZE account_owners; ANALYZE account_balance_projections`); err != nil {
		t.Fatal(err)
	}

	repository, err := db.NewAccountRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page, pageErr := repository.ListOwnedPage(ctx, tenantID, "scale-operator", accounts.Query{Cursor: cursor, Limit: 100})
		if pageErr != nil {
			t.Fatal(pageErr)
		}
		for _, account := range page.Accounts {
			if seen[account.AccountID] {
				t.Fatalf("stable cursor repeated account %s", account.AccountID)
			}
			seen[account.AccountID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 10_000 {
		t.Fatalf("paged account count=%d, want 10000", len(seen))
	}

	filtered, err := repository.ListOwnedPage(ctx, tenantID, "scale-operator", accounts.Query{Limit: 25, Search: "Scale account 0002", Status: "active", Category: "operating"})
	if err != nil || len(filtered.Accounts) == 0 {
		t.Fatalf("filtered page=%d error=%v", len(filtered.Accounts), err)
	}
	for _, account := range filtered.Accounts {
		if account.Status != "active" || account.Category != "operating" || !strings.HasPrefix(strings.ToLower(account.DisplayName), "scale account 0002") {
			t.Fatalf("filter returned unexpected account: %+v", account)
		}
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,occurred_at)VALUES('00000000-0000-0000-0000-000000009010',$1,'scale-operator','account.reviewed','account',$2,'succeeded','00000000-0000-0000-0000-000000009011','2026-08-24T12:00:00Z')`, tenantID, filtered.Accounts[0].AccountID); err != nil {
		t.Fatal(err)
	}
	detail, err := repository.GetOwned(ctx, tenantID, "scale-operator", filtered.Accounts[0].AccountID)
	if err != nil || len(detail.AuditContext) != 1 || detail.AuditContext[0].CorrelationID != "00000000-0000-0000-0000-000000009011" {
		t.Fatalf("account audit context=%+v error=%v", detail.AuditContext, err)
	}
	wildcard, err := repository.ListOwnedPage(ctx, tenantID, "scale-operator", accounts.Query{Limit: 25, Search: "%"})
	if err != nil || len(wildcard.Accounts) != 0 {
		t.Fatalf("search wildcard was not treated literally: count=%d error=%v", len(wildcard.Accounts), err)
	}
	if _, err := repository.GetOwned(ctx, tenantID, "different-subject", filtered.Accounts[0].AccountID); !errors.Is(err, accounts.ErrAccountNotFound) {
		t.Fatalf("object lookup disclosed an inaccessible account: %v", err)
	}

	rows, err := database.QueryContext(ctx, `EXPLAIN (FORMAT TEXT) SELECT a.id FROM accounts a JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id JOIN account_balance_projections b ON b.account_id=a.id WHERE a.tenant_id=$1 AND owner.subject_id='scale-operator' AND owner.permission IN ('read','debit') ORDER BY a.created_at,a.id LIMIT 101`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if strings.Contains(plan.String(), "Seq Scan on accounts") || !strings.Contains(plan.String(), "accounts_tenant_created_stable_idx") {
		t.Fatalf("account directory plan is not using the stable tenant index:\n%s", plan.String())
	}
}

func TestAccountListContainsOnlyAccountsOwnedByCaller(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	repository, err := db.NewAccountRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := repository.ListOwned(context.Background(), testTenantID, testActorID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].AccountID != testSourceID {
		t.Fatalf("owned account list disclosed unexpected account: %#v", accounts)
	}
}
