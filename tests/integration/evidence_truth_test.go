package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/delivery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestReconciliationPersistsMissingProjectionEvidenceAndNeverFalsePasses(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	if _, err := database.ExecContext(context.Background(), `DELETE FROM account_balance_projections WHERE account_id=$1`, testDestinationID); err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewReconciliationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repository.Reconcile(context.Background(), testTenantID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != reconciliation.StatusMismatch || result.CheckedAccountCount != 2 || result.MismatchCount < 1 || result.LedgerWatermark == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	var classification string
	if err := database.QueryRowContext(context.Background(), `SELECT classification FROM reconciliation_mismatches WHERE run_id=$1 AND account_id=$2`, result.ID, testDestinationID).Scan(&classification); err != nil {
		t.Fatal(err)
	}
	if classification != "projection_missing" {
		t.Fatalf("classification=%q", classification)
	}
	investigation, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := investigation.GetReconciliationRun(context.Background(), testTenantID, result.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.MismatchCount == "0" || len(detail.Mismatches) == 0 || detail.Mismatches[0].Classification != "projection_missing" {
		t.Fatalf("mismatch evidence not exposed: %#v", detail)
	}
}

func TestReconciliationTreatsUnintendedEmptyScopeAsMismatch(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	const emptyTenant = "00000000-0000-4000-8000-000000000099"
	if _, err := database.ExecContext(context.Background(), `INSERT INTO tenants (id,external_reference) VALUES ($1,'empty-reconciliation-tenant')`, emptyTenant); err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewReconciliationRepository(database)
	result, err := repository.Reconcile(context.Background(), emptyTenant, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status == reconciliation.StatusMatched || result.CheckedAccountCount != 0 || result.MismatchCount != 1 {
		t.Fatalf("empty scope false-passed: %#v", result)
	}
}

func TestTransferDeliveryStatusRequiresDurableAttemptEvidence(t *testing.T) {
	service, database := requireTransferService(t, 10_000)
	submission, err := service.Submit(context.Background(), transferCommand(t, "delivery-evidence-key-0001", "1.00"))
	if err != nil {
		t.Fatal(err)
	}
	investigation, _ := db.NewInvestigationRepository(database)
	detail, err := investigation.GetTransfer(context.Background(), testTenantID, submission.Result.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.DeliveryStatus != "not_applicable" {
		t.Fatalf("delivery without attempt=%q", detail.DeliveryStatus)
	}

	deliveryRepository, _ := db.NewDeliveryRepository(database)
	if err := deliveryRepository.Record(context.Background(), delivery.Attempt{ID: "00000000-0000-4000-8000-000000000088", TenantID: testTenantID, TransferID: submission.Result.TransferID, Kind: "webhook", EndpointReference: "partner-primary", AttemptNumber: 1, Status: delivery.StatusRetrying, DueAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	detail, err = investigation.GetTransfer(context.Background(), testTenantID, submission.Result.TransferID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.DeliveryStatus != "retrying" {
		t.Fatalf("durable delivery status=%q", detail.DeliveryStatus)
	}
}
