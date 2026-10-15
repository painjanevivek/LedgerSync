package integration_test

import (
	"context"
	"testing"
	"time"

	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestProvisioningRollbackRejectsFundingActivityAndWrongWorkload(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	provisioner := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_provisioning")
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	repository, _ := db.NewProvisioningRepository(provisioner.db, time.Now)
	configuration := partnerProvisioningFixture()
	const correlation = "00000000-0000-0000-0000-000000000302"
	ctx := context.Background()
	if err := repository.Apply(ctx, configuration, "USD", "platform-owner", correlation); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',true,1,100000,200000,500000)`, configuration.TenantID); err != nil {
		t.Fatal(err)
	}
	fundingRepository, _ := db.NewFundingRepository(api.db, time.Now)
	fundingService, _ := appfunding.NewService(fundingRepository, appfunding.PolicyProductionDualControl, time.Now)
	amount, _ := money.New("USD", 100)
	if _, err := fundingService.Request(ctx, appfunding.RequestCommand{TenantID: configuration.TenantID, ActorSubjectID: "partner-operator", DestinationAccountID: configuration.Accounts[0].ID, Amount: amount, ExternalReference: "synthetic-provision-funding", EvidenceReference: "evidence://synthetic/provision", IdempotencyKey: "provision-funding-0001", CorrelationID: correlation}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Rollback(ctx, configuration.TenantID, "platform-owner", correlation); sqlState(err) != "23514" {
		t.Fatalf("rollback with funding: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM accounts WHERE tenant_id=$1 AND status='closed'`, configuration.TenantID); got != 0 {
		t.Fatal("denied rollback closed accounts")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM partner_credential_events WHERE tenant_id=$1 AND action='revoked'`, configuration.TenantID); got != 0 {
		t.Fatal("denied rollback revoked credentials")
	}
	if got := countRows(t, admin, `SELECT count(*) FROM funding_events WHERE tenant_id=$1 AND status='requested'`, configuration.TenantID); got != 1 {
		t.Fatal("funding evidence changed")
	}
	if _, err := tenantTestExec(ctx, api.db, configuration.TenantID, "partner-operator", `SELECT controlled_rollback_partner_v1($1,$2,$3,now())`, configuration.TenantID, "partner-operator", correlation); sqlState(err) != "42501" {
		t.Fatalf("API workload invoked provisioning rollback: %v", err)
	}
}
