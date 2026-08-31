package integration_test

import (
	"context"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTransferHistoryFiltersApplyBeforePaginationAndBindCursorIntent(t *testing.T) {
	service, database := requireTransferService(t, 100_000)
	postedIDs := make([]string, 0, 3)
	for index, key := range []string{"history-filter-posted-0001", "history-filter-posted-0002", "history-filter-posted-0003"} {
		submission, err := service.Submit(context.Background(), transferCommand(t, key, "0.01"))
		if err != nil || submission.Result.Status != "posted" {
			t.Fatalf("posted transfer %d result=%#v err=%v", index, submission, err)
		}
		postedIDs = append(postedIDs, submission.Result.TransferID)
	}
	rejected, err := service.Submit(context.Background(), transferCommand(t, "history-filter-rejected-01", "1000.01"))
	if err != nil || rejected.Result.Status != "rejected" {
		t.Fatalf("rejected transfer result=%#v err=%v", rejected, err)
	}

	repository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	firstPage, cursor, err := repository.ListTransfers(context.Background(), testTenantID, investigation.TransferFilter{Status: "posted", Limit: 2})
	if err != nil || len(firstPage) != 2 || cursor == "" {
		t.Fatalf("first posted page=%#v cursor=%q err=%v", firstPage, cursor, err)
	}
	secondPage, next, err := repository.ListTransfers(context.Background(), testTenantID, investigation.TransferFilter{Status: "posted", Cursor: cursor, Limit: 2})
	if err != nil || len(secondPage) != 1 || next != "" {
		t.Fatalf("second posted page=%#v cursor=%q err=%v", secondPage, next, err)
	}
	if _, _, err := repository.ListTransfers(context.Background(), testTenantID, investigation.TransferFilter{Status: "rejected", Cursor: cursor, Limit: 2}); err == nil {
		t.Fatal("posted cursor was accepted for a changed status filter")
	}
	query := postedIDs[0][:12]
	matches, _, err := repository.ListTransfers(context.Background(), testTenantID, investigation.TransferFilter{Query: query, Limit: 25})
	if err != nil || len(matches) != 1 || matches[0].ID != postedIDs[0] {
		t.Fatalf("server-side transfer ID query %q returned %#v err=%v", query, matches, err)
	}
}

func TestExactInvestigationSearchRepeatsTenantObjectAndDomainAuthorization(t *testing.T) {
	service, database := requireTransferService(t, 100_000)
	submission, err := service.Submit(context.Background(), transferCommand(t, "exact-search-transfer-0001", "0.01"))
	if err != nil || submission.Result.Status != "posted" {
		t.Fatalf("posted transfer=%#v err=%v", submission, err)
	}
	if _, err := database.ExecContext(context.Background(), `UPDATE accounts SET external_reference='SEARCH-ACCOUNT' WHERE tenant_id=$1 AND id=$2`, testTenantID, testSourceID); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}

	accountPage, err := repository.Search(context.Background(), testTenantID, testActorID, investigation.SearchFilter{Query: testSourceID, QueryKind: "immutable_id", Limit: 10, Access: investigation.SearchAccess{Accounts: true}})
	if err != nil || len(accountPage.Results) != 1 || accountPage.Results[0].RecordType != "account" || accountPage.Results[0].RecordID != testSourceID {
		t.Fatalf("owned account search=%#v err=%v", accountPage, err)
	}
	unauthorizedAccount, err := repository.Search(context.Background(), testTenantID, testActorID, investigation.SearchFilter{Query: testDestinationID, QueryKind: "immutable_id", Limit: 10, Access: investigation.SearchAccess{Accounts: true}})
	if err != nil || len(unauthorizedAccount.Results) != 0 {
		t.Fatalf("non-owned account leaked=%#v err=%v", unauthorizedAccount, err)
	}
	referencePage, err := repository.Search(context.Background(), testTenantID, testActorID, investigation.SearchFilter{Query: "SEARCH-ACCOUNT", QueryKind: "approved_reference", Limit: 10, Access: investigation.SearchAccess{Accounts: true}})
	if err != nil || len(referencePage.Results) != 1 || referencePage.Results[0].RecordID != testSourceID {
		t.Fatalf("exact account reference search=%#v err=%v", referencePage, err)
	}
	partialPage, err := repository.Search(context.Background(), testTenantID, testActorID, investigation.SearchFilter{Query: "SEARCH-A", QueryKind: "approved_reference", Limit: 10, Access: investigation.SearchAccess{Accounts: true}})
	if err != nil || len(partialPage.Results) != 0 {
		t.Fatalf("partial reference unexpectedly matched=%#v err=%v", partialPage, err)
	}
	transferPage, err := repository.Search(context.Background(), testTenantID, testActorID, investigation.SearchFilter{Query: submission.Result.TransferID, QueryKind: "immutable_id", Limit: 10, Access: investigation.SearchAccess{Transfers: true}})
	if err != nil || len(transferPage.Results) != 1 || transferPage.Results[0].RecordType != "transfer" {
		t.Fatalf("transfer search=%#v err=%v", transferPage, err)
	}
	domainDenied, err := repository.Search(context.Background(), testTenantID, testActorID, investigation.SearchFilter{Query: submission.Result.TransferID, QueryKind: "immutable_id", Limit: 10, Access: investigation.SearchAccess{Accounts: true}})
	if err != nil || len(domainDenied.Results) != 0 {
		t.Fatalf("transfer leaked through account scope=%#v err=%v", domainDenied, err)
	}
}
