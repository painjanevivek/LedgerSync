package integration_test

import (
	"context"
	"strings"
	"testing"
)

func TestControlledFunctionOwnersPathsAndDefaultPrivileges(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	rows, err := admin.Query(`SELECT p.proname,r.rolname,r.rolcanlogin,r.rolsuper,r.rolbypassrls,p.prosecdef,
 COALESCE((SELECT c FROM unnest(p.proconfig)c WHERE c LIKE 'search_path=%'),''),
 EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner)))a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_roles r ON r.oid=p.proowner WHERE n.nspname='public' AND p.proname LIKE 'controlled_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	owners := map[string]bool{"ledgersync_migration_owner": true, "ledgersync_queue_owner": true, "ledgersync_preflight_owner": true, "ledgersync_maintenance_owner": true}
	for rows.Next() {
		var name, owner, path string
		var login, super, bypass, definer, public bool
		if err := rows.Scan(&name, &owner, &login, &super, &bypass, &definer, &path, &public); err != nil {
			t.Fatal(err)
		}
		count++
		if !owners[owner] || login || super || bypass || public || !strings.HasPrefix(path, "search_path=pg_catalog, public") || (!definer && name != "controlled_fingerprint_parts_v1") {
			t.Errorf("unsafe controlled function %s owner=%s path=%s public=%t", name, owner, path, public)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 25 {
		t.Fatalf("controlled function inventory incomplete: %d", count)
	}
	tx, err := admin.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`CREATE FUNCTION public.portfolio_default_acl_probe() RETURNS integer LANGUAGE sql AS 'SELECT 1'`); err != nil {
		t.Fatal(err)
	}
	var canExecute bool
	if err := tx.QueryRow(`SELECT has_function_privilege('ledgersync_api','public.portfolio_default_acl_probe()','EXECUTE')`).Scan(&canExecute); err != nil {
		t.Fatal(err)
	}
	if canExecute {
		t.Fatal("new functions regain PUBLIC execution by default")
	}
}
