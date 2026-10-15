package integration_test

import (
	"context"
	"testing"
)

func TestControlledTransferBindsDeclaredActorToTransactionScope(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	const peer = "synthetic-authorized-peer"
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'operator')`, []any{testTenantID, peer}},
		{`INSERT INTO account_owners(tenant_id,account_id,subject_id,permission) VALUES($1,$2,$3,'debit')`, []any{testTenantID, testSourceID, peer}},
		{`INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id) VALUES($1,$2,$3)`, []any{testTenantID, testDestinationID, peer}},
	} {
		if _, err := admin.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	command := transferCommand(t, "controlled-peer-spoof-0001", "1.00")
	command.ActorSubjectID = peer
	fingerprint := transferFingerprint(t, command)
	_, err := tenantTestExec(context.Background(), api.db, testTenantID, testActorID, `SELECT * FROM public.controlled_submit_transfer_v1($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, command.TenantID, command.ActorSubjectID, command.DebitAccountID, command.CreditAccountID, command.Amount.Minor(), command.Amount.Currency().Code, command.IdempotencyKey, fingerprint, command.CorrelationID, "USD", controlledTransferOccurredAt)
	if sqlState(err) != "42501" {
		t.Fatalf("authorized peer substituted for scoped actor: %v", err)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM transfers`); got != 0 {
		t.Fatal("denied actor substitution posted a transfer")
	}
}
