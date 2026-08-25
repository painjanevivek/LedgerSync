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
