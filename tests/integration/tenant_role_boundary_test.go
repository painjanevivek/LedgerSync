package integration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestTenantRoleProvisioningRejectsUnsafeInheritance(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	roles, err := os.ReadFile(filepath.Join(liveMigrationRoot(t), "deploy", "postgres", "roles.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`ALTER ROLE ledgersync_api BYPASSRLS`,
		`GRANT ledgersync_queue_owner TO ledgersync_worker`,
		`GRANT ledgersync_preflight_owner TO ledgersync_support_readonly`,
	} {
		t.Run(statement, func(t *testing.T) {
			tx, err := admin.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.Exec(statement); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(string(roles)); sqlState(err) != "42501" {
				t.Fatalf("unsafe role setup was not rejected: %v", err)
			}
		})
	}
}

func TestTenantForcedPolicyAppliesToNonSuperuserOwner(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	owner := pgx.Identifier{session.login}.Sanitize()
	if _, err := admin.Exec(`CREATE TABLE portfolio_owner_scope(tenant_id uuid,note text);
 INSERT INTO portfolio_owner_scope VALUES('00000000-0000-4000-8000-000000000001','synthetic');
 ALTER TABLE portfolio_owner_scope ENABLE ROW LEVEL SECURITY;
 ALTER TABLE portfolio_owner_scope FORCE ROW LEVEL SECURITY;
 CREATE POLICY tenant_scope ON portfolio_owner_scope USING(public.tenant_scope_allows(tenant_id));
 GRANT CREATE ON SCHEMA public TO ` + owner + `; ALTER TABLE portfolio_owner_scope OWNER TO ` + owner + `; REVOKE CREATE ON SCHEMA public FROM ` + owner); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP TABLE portfolio_owner_scope`); err != nil {
			t.Error(err)
		}
	})
	var count int
	if err := session.db.QueryRow(`SELECT count(*) FROM portfolio_owner_scope`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("owner bypassed forced policy: count=%d err=%v", count, err)
	}
	if err := db.WithTenantTransaction(context.Background(), session.db, db.TenantScope{TenantID: "00000000-0000-4000-8000-000000000001", ActorID: testActorID}, nil, func(tx *sql.Tx) error { return tx.QueryRow(`SELECT count(*) FROM portfolio_owner_scope`).Scan(&count) }); err != nil || count != 1 {
		t.Fatalf("scoped owner lost permitted row: count=%d err=%v", count, err)
	}
}

func TestTenantPermanentPolicyRejectsMalformedContextAndForeignJoin(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	session := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	for _, tenant := range []string{"", "not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		tx, err := session.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(`SELECT set_config('app.tenant_id',$1,true),set_config('app.actor_id',$2,true)`, tenant, testActorID); err != nil {
			t.Fatal(err)
		}
		var count int
		err = tx.QueryRow(`SELECT count(*) FROM accounts`).Scan(&count)
		_ = tx.Rollback()
		if err != nil || count != 0 {
			t.Fatalf("malformed scope exposed rows: %v", err)
		}
	}
	const other = "00000000-0000-4000-8000-000000000982"
	const account = "00000000-0000-4000-8000-000000000983"
	if _, err := admin.Exec(`INSERT INTO tenants(id,external_reference) VALUES($1,'synthetic-policy-other')`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference) VALUES($1,$2,'USD','active','Synthetic other','operating','synthetic-policy-other')`, account, other); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version) VALUES($1,0,0,0)`, account); err != nil {
		t.Fatal(err)
	}
	err := db.WithTenantTransaction(context.Background(), session.db, db.TenantScope{TenantID: testTenantID, ActorID: testActorID}, nil, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRow(`SELECT count(*) FROM accounts a JOIN account_balance_projections b ON a.id=b.account_id WHERE a.tenant_id<>$1`, testTenantID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Fatal("foreign join exposed a row")
		}
		if err := tx.QueryRow(`SELECT count(*) FROM account_balance_projections WHERE account_id=$1`, account).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Fatal("direct projection ID exposed a foreign row")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
