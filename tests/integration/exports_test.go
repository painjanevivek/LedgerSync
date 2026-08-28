package integration_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appexports "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/exports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestPostgreSQLExportsAreTenantScopedObjectAuthorizedAndExact(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	submission, err := transferService.Submit(context.Background(), transferCommand(t, "phase7-export-key-0001", "25.00"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewExportRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := appexports.NewService(repository, appexports.DefaultMaxRows, 1)
	if err != nil {
		t.Fatal(err)
	}

	var transfersCSV bytes.Buffer
	result, err := service.StreamTransfers(context.Background(), testTenantID, investigation.TransferFilter{Status: "posted"}, 10, &transfersCSV)
	if err != nil || result.Rows != 1 || !strings.Contains(transfersCSV.String(), `"`+submission.Result.TransferID+`"`) || !strings.Contains(transfersCSV.String(), `"2500","USD"`) {
		t.Fatalf("transfer export result=%+v error=%v csv=%s", result, err, transfersCSV.String())
	}
	var otherTenantCSV bytes.Buffer
	other, err := service.StreamTransfers(context.Background(), "00000000-0000-4000-8000-000000000099", investigation.TransferFilter{}, 10, &otherTenantCSV)
	if err != nil || other.Rows != 0 || strings.Contains(otherTenantCSV.String(), submission.Result.TransferID) {
		t.Fatalf("cross-tenant export result=%+v error=%v csv=%s", other, err, otherTenantCSV.String())
	}

	var accountCSV bytes.Buffer
	accountResult, err := service.StreamAccountLedger(context.Background(), testTenantID, testActorID, testSourceID, 10, &accountCSV)
	if err != nil || accountResult.Rows != 1 || !strings.Contains(accountCSV.String(), `"debit","2500","USD"`) {
		t.Fatalf("account export result=%+v error=%v csv=%s", accountResult, err, accountCSV.String())
	}
	if _, err := service.StreamAccountLedger(context.Background(), testTenantID, testActorID, testDestinationID, 10, &bytes.Buffer{}); !errors.Is(err, transactions.ErrHistoryNotFound) {
		t.Fatalf("credit-only destination account disclosed ledger: %v", err)
	}
}

func TestPostgreSQLReconciliationExportIncludesBoundedMismatchEvidence(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	if _, err := database.ExecContext(context.Background(), `DELETE FROM account_balance_projections WHERE account_id=$1`, testDestinationID); err != nil {
		t.Fatal(err)
	}
	reconciliationRepository, err := db.NewReconciliationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	run, err := reconciliationRepository.Reconcile(context.Background(), testTenantID, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	exportRepository, err := db.NewExportRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := appexports.NewService(exportRepository, appexports.DefaultMaxRows, 1)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	result, err := service.StreamReconciliation(context.Background(), testTenantID, appexports.ReconciliationFilter{RunID: run.ID, Status: "mismatch"}, 10, &output)
	if err != nil || result.Rows < 2 || !strings.Contains(output.String(), `"run","`+run.ID+`","mismatch"`) || !strings.Contains(output.String(), `"mismatch"`) || !strings.Contains(output.String(), `"projection_missing"`) {
		t.Fatalf("reconciliation export result=%+v error=%v csv=%s", result, err, output.String())
	}
}
