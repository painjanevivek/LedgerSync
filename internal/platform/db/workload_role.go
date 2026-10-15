package db

import (
	"context"
	"database/sql"
	"errors"
)

var ErrWorkloadRoleBinding = errors.New("database login does not have the required isolated workload role")

// RequireWorkloadRole rejects owner/superuser credentials, extra inherited
// roles and a mismatched workload. Provisioning must bind a separate LOGIN to
// exactly one approved NOLOGIN role before optional worker execution starts.
func RequireWorkloadRole(ctx context.Context, database *sql.DB, role string) error {
	switch role {
	case "ledgersync_api", "ledgersync_worker", "ledgersync_reconciliation", "ledgersync_scheduler", "ledgersync_provisioning", "ledgersync_support_readonly", "ledgersync_maintenance":
	default:
		return ErrWorkloadRoleBinding
	}
	if database == nil {
		return ErrWorkloadRoleBinding
	}
	var valid bool
	err := database.QueryRowContext(ctx, `SELECT current_user=session_user AND login.rolcanlogin AND login.rolinherit AND NOT login.rolsuper AND NOT login.rolbypassrls AND NOT login.rolcreaterole AND NOT login.rolcreatedb AND NOT login.rolreplication
 AND EXISTS(SELECT 1 FROM pg_roles expected WHERE expected.rolname=$1 AND NOT expected.rolcanlogin AND NOT expected.rolsuper AND NOT expected.rolbypassrls AND NOT expected.rolcreaterole AND NOT expected.rolcreatedb AND NOT expected.rolreplication AND pg_has_role(session_user,expected.oid,'MEMBER'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles extra WHERE extra.rolname<>session_user AND extra.rolname<>$1 AND pg_has_role(session_user,extra.oid,'MEMBER'))
 AND NOT EXISTS(SELECT 1 FROM pg_database WHERE datname=current_database() AND datdba=login.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_class relation JOIN pg_namespace ns ON ns.oid=relation.relnamespace WHERE ns.nspname='public' AND relation.relowner=login.oid)
 FROM pg_roles login WHERE login.rolname=session_user`, role).Scan(&valid)
	if err != nil {
		return errors.Join(ErrWorkloadRoleBinding, err)
	}
	if !valid {
		return ErrWorkloadRoleBinding
	}
	return nil
}
