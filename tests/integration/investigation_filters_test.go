package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestSavedInvestigationViewsAreOwnerScopedVersionedAndAtomicallyAudited(t *testing.T) {
	_, database := requireTransferService(t, 100_000)
	repository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	created, err := repository.CreateSavedView(context.Background(), investigation.SavedViewCreate{
		TenantID: testTenantID, ActorID: testActorID, Name: "Frozen accounts", Domain: "accounts",
		FilterSchemaVersion: 1, Filters: map[string]string{"status": "frozen"}, Access: investigation.SavedViewAccess{Accounts: true},
		CorrelationID: "00000000-0000-4000-8000-000000000931", OccurredAt: time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
	})
	if err != nil || created.Version != "1" || created.TargetPath != "/accounts?status=frozen" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	page, err := repository.ListSavedViews(context.Background(), testTenantID, testActorID, investigation.SavedViewAccess{Accounts: true})
	if err != nil || len(page.Views) != 1 || page.Views[0].ID != created.ID {
		t.Fatalf("owner page=%#v err=%v", page, err)
	}
	other, err := repository.ListSavedViews(context.Background(), testTenantID, "other-operator", investigation.SavedViewAccess{Accounts: true})
	if err != nil || len(other.Views) != 0 {
		t.Fatalf("cross-operator page=%#v err=%v", other, err)
	}
	hidden, err := repository.ListSavedViews(context.Background(), testTenantID, testActorID, investigation.SavedViewAccess{Transfers: true})
	if err != nil || len(hidden.Views) != 0 {
		t.Fatalf("domain-revoked page=%#v err=%v", hidden, err)
	}
	if _, err := repository.RenameSavedView(context.Background(), investigation.SavedViewRename{TenantID: testTenantID, ActorID: testActorID, SavedViewID: created.ID, Name: "Hidden rename", ExpectedVersion: 1, Access: investigation.SavedViewAccess{Transfers: true}, CorrelationID: "00000000-0000-4000-8000-000000000930"}); !errors.Is(err, investigation.ErrSavedViewNotFound) {
		t.Fatalf("domain-revoked rename err=%v", err)
	}

	renamed, err := repository.RenameSavedView(context.Background(), investigation.SavedViewRename{
		TenantID: testTenantID, ActorID: testActorID, SavedViewID: created.ID, Name: "Accounts requiring review", ExpectedVersion: 1,
		Access: investigation.SavedViewAccess{Accounts: true}, CorrelationID: "00000000-0000-4000-8000-000000000932", OccurredAt: time.Date(2026, 8, 31, 12, 1, 0, 0, time.UTC),
	})
	if err != nil || renamed.Version != "2" || renamed.Name != "Accounts requiring review" {
		t.Fatalf("renamed=%#v err=%v", renamed, err)
	}
	if _, err := repository.RenameSavedView(context.Background(), investigation.SavedViewRename{TenantID: testTenantID, ActorID: testActorID, SavedViewID: created.ID, Name: "Stale rename", ExpectedVersion: 1, Access: investigation.SavedViewAccess{Accounts: true}, CorrelationID: "00000000-0000-4000-8000-000000000933"}); !errors.Is(err, investigation.ErrSavedViewVersion) {
		t.Fatalf("stale rename err=%v", err)
	}
	if err := repository.DeleteSavedView(context.Background(), investigation.SavedViewDelete{TenantID: testTenantID, ActorID: testActorID, SavedViewID: created.ID, ExpectedVersion: 2, Access: investigation.SavedViewAccess{Transfers: true}, CorrelationID: "00000000-0000-4000-8000-000000000935"}); !errors.Is(err, investigation.ErrSavedViewNotFound) {
		t.Fatalf("domain-revoked delete err=%v", err)
	}
	if err := repository.DeleteSavedView(context.Background(), investigation.SavedViewDelete{TenantID: testTenantID, ActorID: testActorID, SavedViewID: created.ID, ExpectedVersion: 2, Access: investigation.SavedViewAccess{Accounts: true}, CorrelationID: "00000000-0000-4000-8000-000000000934", OccurredAt: time.Date(2026, 8, 31, 12, 2, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM audit_events WHERE tenant_id=$1 AND target_id=$2 AND event_type IN ('investigation.saved_view_created','investigation.saved_view_renamed','investigation.saved_view_deleted')`, testTenantID, created.ID); got != 3 {
		t.Fatalf("saved-view audit events=%d, want 3", got)
	}
	if got := countRows(t, database, `SELECT count(*) FROM investigation_saved_views WHERE tenant_id=$1 AND owner_subject_id=$2`, testTenantID, testActorID); got != 0 {
		t.Fatalf("saved views after delete=%d", got)
	}
}

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

func TestRelatedEvidenceUsesExplicitKeysAndFiltersTargetDomains(t *testing.T) {
	service, database := requireTransferService(t, 100_000)
	submission, err := service.Submit(context.Background(), transferCommand(t, "related-evidence-transfer-0001", "0.01"))
	if err != nil || submission.Result.Status != "posted" {
		t.Fatalf("posted transfer=%#v err=%v", submission, err)
	}
	repository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	page, err := repository.Related(context.Background(), testTenantID, testActorID, investigation.RelationshipFilter{
		SourceType: "transfer", SourceID: submission.Result.TransferID, Limit: 20,
		Access: investigation.RelationshipAccess{Accounts: true, Transfers: true, Events: true, Reconciliation: true, Corrections: true},
	})
	if err != nil || page.SourceID != submission.Result.TransferID || len(page.Relationships) == 0 || page.Truncated {
		t.Fatalf("related page=%#v err=%v", page, err)
	}
	seen := map[string]bool{}
	for _, relationship := range page.Relationships {
		seen[relationship.RelationshipType] = true
		if relationship.Source != "postgresql" || relationship.Freshness != "relationship_snapshot" {
			t.Fatalf("relationship provenance=%#v", relationship)
		}
		if relationship.TargetType == "account" && relationship.TargetID == testDestinationID {
			t.Fatalf("non-owned destination account leaked through relationship edge: %#v", relationship)
		}
	}
	if !seen["transfer_journal"] || !seen["journal_posting"] || !seen["transfer_source_account"] || !seen["transfer_event"] {
		t.Fatalf("required explicit relationships missing: %#v", seen)
	}
	_, err = repository.Related(context.Background(), testTenantID, testActorID, investigation.RelationshipFilter{SourceType: "transfer", SourceID: submission.Result.TransferID, Limit: 20, Access: investigation.RelationshipAccess{Accounts: true}})
	if !errors.Is(err, db.ErrInvestigationNotFound) {
		t.Fatalf("transfer source was disclosed without transfers:read: %v", err)
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
