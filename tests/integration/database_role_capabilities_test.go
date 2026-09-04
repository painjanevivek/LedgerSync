package integration_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

const databaseCapabilityMatrixVersion = "2026-09-03.pr005.v1"

var (
	capabilityRoles = []string{
		"ledgersync_api",
		"ledgersync_worker",
		"ledgersync_reconciliation",
		"ledgersync_provisioning",
		"ledgersync_support_readonly",
		"ledgersync_break_glass",
	}
	capabilityTables = []string{
		"accounts",
		"account_opening_balances",
		"account_balance_projections",
		"journal_transactions",
		"ledger_postings",
		"account_owners",
		"audit_events",
	}
	tableUpdateColumns = map[string]string{
		"accounts":                    "status",
		"account_opening_balances":    "opening_ledger_minor",
		"account_balance_projections": "available_minor",
		"journal_transactions":        "occurred_at",
		"ledger_postings":             "occurred_at",
		"account_owners":              "permission",
		"audit_events":                "outcome",
	}
)

type capabilityExpectation struct {
	Role           string
	Capability     string
	CurrentAllowed bool
	TargetAllowed  bool
}

type capabilityEvidence struct {
	MatrixVersion  string `json:"matrix_version"`
	Installation   string `json:"installation"`
	Role           string `json:"role"`
	Capability     string `json:"capability"`
	ActualAllowed  bool   `json:"actual_allowed"`
	CurrentAllowed bool   `json:"current_allowed"`
	TargetAllowed  bool   `json:"target_allowed"`
	Status         string `json:"status"`
}

type workloadSession struct {
	login string
	group string
	db    *sql.DB
}

func TestDatabaseRoleCapabilities(t *testing.T) {
	_, admin := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, admin)

	const otherTenantID = "00000000-0000-4000-8000-000000000902"
	const otherAccountID = "00000000-0000-4000-8000-000000000903"
	if _, err := admin.ExecContext(context.Background(), `INSERT INTO tenants(id,external_reference) VALUES($1,'capability-other-tenant')`, otherTenantID); err != nil {
		t.Fatalf("seed cross-tenant capability tenant: %v", err)
	}
	if _, err := admin.ExecContext(context.Background(), `
INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference)
VALUES($2,$1,'USD','active','Capability isolation account','operating','capability-isolation')`, otherTenantID, otherAccountID); err != nil {
		t.Fatalf("seed cross-tenant capability fixture: %v", err)
	}

	evidence := verifyDatabaseRoleCapabilities(t, admin, os.Getenv("LEDGERSYNC_TEST_DATABASE_URL"), "fresh-install", testTenantID, otherTenantID)
	if reportPath := os.Getenv("LEDGERSYNC_CAPABILITY_REPORT_PATH"); reportPath != "" {
		writeCapabilityReport(t, reportPath, evidence)
	}
}

func verifyDatabaseRoleCapabilities(t *testing.T, admin *sql.DB, databaseURL, installation, sessionTenantID, otherTenantID string) []capabilityEvidence {
	t.Helper()
	requireWorkloadRoles(t, admin)
	expectations := databaseCapabilityExpectations()
	evidence := make([]capabilityEvidence, 0, len(expectations))

	for _, role := range capabilityRoles {
		role := role
		t.Run(installation+"/"+role, func(t *testing.T) {
			session := provisionWorkloadSession(t, admin, databaseURL, role)
			verifyWorkloadSession(t, admin, session, sessionTenantID)

			for _, expectation := range expectations {
				if expectation.Role != role {
					continue
				}
				actual := probeCapability(t, session.db, expectation.Capability, otherTenantID)
				row := newCapabilityEvidence(installation, expectation, actual)
				evidence = append(evidence, row)
				if actual != expectation.CurrentAllowed {
					t.Errorf("capability drift: %s actual=%t current=%t target=%t", expectation.Capability, actual, expectation.CurrentAllowed, expectation.TargetAllowed)
				}
			}
		})
	}

	t.Run(installation+"/owner-positive-control", func(t *testing.T) {
		for _, capability := range []string{
			"table.accounts.SELECT",
			"table.accounts.INSERT",
			"table.accounts.UPDATE",
			"table.accounts.DELETE",
			"cross_tenant.accounts.SELECT",
			"function.reject_ledger_mutation.EXECUTE",
			"function.controlled_submit_transfer_v1.EXECUTE",
			"function.controlled_post_funding_v1.EXECUTE",
			"function.controlled_post_transfer_correction_v1.EXECUTE",
			"function.controlled_provision_account_v1.EXECUTE",
			"database.schema.CREATE",
		} {
			if !probeCapability(t, admin, capability, otherTenantID) {
				t.Errorf("owner positive control could not execute %s", capability)
			}
		}
	})

	sort.Slice(evidence, func(i, j int) bool {
		if evidence[i].Role == evidence[j].Role {
			return evidence[i].Capability < evidence[j].Capability
		}
		return evidence[i].Role < evidence[j].Role
	})
	return evidence
}

func databaseCapabilityExpectations() []capabilityExpectation {
	current := map[string]map[string]string{
		"ledgersync_api": {
			"accounts": "SIU", "account_opening_balances": "SI", "account_balance_projections": "SIU",
			"journal_transactions": "SI", "ledger_postings": "SI", "account_owners": "SI", "audit_events": "SI",
		},
		"ledgersync_worker": {"audit_events": "I"},
		"ledgersync_reconciliation": {
			"accounts": "S", "account_opening_balances": "S", "account_balance_projections": "S",
			"journal_transactions": "S", "ledger_postings": "S", "audit_events": "I",
		},
		"ledgersync_provisioning": {
			"accounts": "IU", "account_opening_balances": "I", "account_balance_projections": "I",
			"account_owners": "ID", "audit_events": "I",
		},
		"ledgersync_support_readonly": {
			"accounts": "S", "account_balance_projections": "S", "journal_transactions": "S",
			"ledger_postings": "S", "account_owners": "S", "audit_events": "S",
		},
		"ledgersync_break_glass": {},
	}
	targetReads := map[string]map[string]bool{
		"ledgersync_api": {
			"accounts": true, "account_opening_balances": true, "account_balance_projections": true,
			"journal_transactions": true, "ledger_postings": true, "account_owners": true, "audit_events": true,
		},
		"ledgersync_worker": {},
		"ledgersync_reconciliation": {
			"accounts": true, "account_opening_balances": true, "account_balance_projections": true,
			"journal_transactions": true, "ledger_postings": true,
		},
		"ledgersync_provisioning": {},
		"ledgersync_support_readonly": {
			"accounts": true, "account_balance_projections": true, "journal_transactions": true,
			"ledger_postings": true, "account_owners": true, "audit_events": true,
		},
		"ledgersync_break_glass": {},
	}
	privileges := []struct {
		name, code string
	}{{"SELECT", "S"}, {"INSERT", "I"}, {"UPDATE", "U"}, {"DELETE", "D"}}

	expectations := make([]capabilityExpectation, 0, len(capabilityRoles)*(len(capabilityTables)*4+3))
	for _, role := range capabilityRoles {
		for _, table := range capabilityTables {
			for _, privilege := range privileges {
				expectations = append(expectations, capabilityExpectation{
					Role:           role,
					Capability:     "table." + table + "." + privilege.name,
					CurrentAllowed: strings.Contains(current[role][table], privilege.code),
					TargetAllowed:  privilege.name == "SELECT" && targetReads[role][table],
				})
			}
		}
		expectations = append(expectations,
			capabilityExpectation{Role: role, Capability: "cross_tenant.accounts.SELECT", CurrentAllowed: strings.Contains(current[role]["accounts"], "S"), TargetAllowed: false},
			capabilityExpectation{Role: role, Capability: "function.reject_ledger_mutation.EXECUTE", CurrentAllowed: role != "ledgersync_break_glass", TargetAllowed: false},
			capabilityExpectation{Role: role, Capability: "function.controlled_submit_transfer_v1.EXECUTE", CurrentAllowed: role == "ledgersync_api", TargetAllowed: role == "ledgersync_api"},
			capabilityExpectation{Role: role, Capability: "function.controlled_post_funding_v1.EXECUTE", CurrentAllowed: role == "ledgersync_api", TargetAllowed: role == "ledgersync_api"},
			capabilityExpectation{Role: role, Capability: "function.controlled_post_transfer_correction_v1.EXECUTE", CurrentAllowed: role == "ledgersync_api", TargetAllowed: role == "ledgersync_api"},
			capabilityExpectation{Role: role, Capability: "function.controlled_provision_account_v1.EXECUTE", CurrentAllowed: role == "ledgersync_api" || role == "ledgersync_provisioning", TargetAllowed: role == "ledgersync_api" || role == "ledgersync_provisioning"},
			capabilityExpectation{Role: role, Capability: "database.schema.CREATE", CurrentAllowed: false, TargetAllowed: false},
		)
	}
	return expectations
}

func provisionWorkloadSession(t *testing.T, admin *sql.DB, databaseURL, group string) *workloadSession {
	t.Helper()
	suffix := randomHex(t, 8)
	password := randomHex(t, 32)
	login := "ledgersync_capability_" + suffix
	quotedLogin := pgx.Identifier{login}.Sanitize()
	quotedGroup := pgx.Identifier{group}.Sanitize()

	if _, err := admin.ExecContext(context.Background(), `CREATE ROLE `+quotedLogin+` LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '`+password+`'`); err != nil {
		t.Fatalf("create ephemeral capability login: %v", redactSecret(err, password))
	}
	if _, err := admin.ExecContext(context.Background(), `GRANT `+quotedGroup+` TO `+quotedLogin); err != nil {
		_, _ = admin.ExecContext(context.Background(), `DROP ROLE IF EXISTS `+quotedLogin)
		t.Fatalf("grant workload role to ephemeral login: %v", redactSecret(err, password))
	}

	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse capability database URL: %v", err)
	}
	parsed.User = url.UserPassword(login, password)
	limited, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: parsed.String()})
	if err != nil {
		_, _ = admin.ExecContext(context.Background(), `DROP ROLE IF EXISTS `+quotedLogin)
		t.Fatalf("open ephemeral capability session: %v", redactSecret(err, password))
	}
	t.Cleanup(func() {
		if err := limited.Close(); err != nil {
			t.Errorf("close ephemeral %s session: %v", group, err)
		}
		if _, err := admin.ExecContext(context.Background(), `DROP ROLE IF EXISTS `+quotedLogin); err != nil {
			t.Errorf("drop ephemeral %s login: %v", group, err)
		}
	})
	return &workloadSession{login: login, group: group, db: limited}
}

func verifyWorkloadSession(t *testing.T, admin *sql.DB, session *workloadSession, tenantID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := session.db.ExecContext(ctx, `SELECT set_config('application_name','ledgersync-capability-harness',false), set_config('app.tenant_id',$1,false), set_config('row_security','on',false)`, tenantID); err != nil {
		t.Fatalf("configure workload session: %v", err)
	}
	var currentUser, sessionUser, rowSecurity string
	if err := session.db.QueryRowContext(ctx, `SELECT current_user,session_user,current_setting('row_security')`).Scan(&currentUser, &sessionUser, &rowSecurity); err != nil {
		t.Fatalf("read workload session identity: %v", err)
	}
	if currentUser != session.login || sessionUser != session.login || rowSecurity != "on" {
		t.Errorf("unexpected workload session identity current=%q session=%q row_security=%q", currentUser, sessionUser, rowSecurity)
	}

	var canLogin, superuser, createDB, createRole, replication, bypassRLS bool
	if err := admin.QueryRowContext(ctx, `SELECT rolcanlogin,rolsuper,rolcreatedb,rolcreaterole,rolreplication,rolbypassrls FROM pg_roles WHERE rolname=$1`, session.login).Scan(&canLogin, &superuser, &createDB, &createRole, &replication, &bypassRLS); err != nil {
		t.Fatalf("read workload login attributes: %v", err)
	}
	if !canLogin || superuser || createDB || createRole || replication || bypassRLS {
		t.Errorf("unsafe workload login attributes for %s", session.group)
	}

	var intended, sibling, directGrants int
	if err := admin.QueryRowContext(ctx, `
SELECT count(*) FILTER (WHERE granted.rolname=$2), count(*) FILTER (WHERE granted.rolname<>$2)
FROM pg_auth_members membership
JOIN pg_roles member ON member.oid=membership.member
JOIN pg_roles granted ON granted.oid=membership.roleid
WHERE member.rolname=$1`, session.login, session.group).Scan(&intended, &sibling); err != nil {
		t.Fatalf("read workload memberships: %v", err)
	}
	if err := admin.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.role_table_grants WHERE grantee=$1`, session.login).Scan(&directGrants); err != nil {
		t.Fatalf("read workload direct grants: %v", err)
	}
	if intended != 1 || sibling != 0 || directGrants != 0 {
		t.Errorf("unsafe workload login binding for %s: intended=%d sibling=%d direct_table_grants=%d", session.group, intended, sibling, directGrants)
	}
}

func requireWorkloadRoles(t *testing.T, admin *sql.DB) {
	t.Helper()
	for _, role := range capabilityRoles {
		var exists bool
		if err := admin.QueryRowContext(context.Background(), `SELECT to_regrole($1) IS NOT NULL`, role).Scan(&exists); err != nil {
			t.Fatalf("resolve workload role %s: %v", role, err)
		}
		if !exists {
			t.Fatalf("workload role %s is missing; apply deploy/postgres/roles.sql before running capability evidence", role)
		}
	}
}

func probeCapability(t *testing.T, database *sql.DB, capability, otherTenantID string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch capability {
	case "cross_tenant.accounts.SELECT":
		var count int
		err := database.QueryRowContext(ctx, `SELECT count(*) FROM public.accounts WHERE tenant_id=$1`, otherTenantID).Scan(&count)
		if err == nil && count < 1 {
			t.Errorf("cross-tenant probe read %d rows, want at least 1", count)
		}
		return classifyPrivilegeProbe(t, capability, err, nil)
	case "function.reject_ledger_mutation.EXECUTE":
		_, err := database.ExecContext(ctx, `SELECT public.reject_ledger_mutation()`)
		return classifyPrivilegeProbe(t, capability, err, map[string]bool{"0A000": true})
	case "function.controlled_submit_transfer_v1.EXECUTE":
		_, err := database.ExecContext(ctx, `SELECT * FROM public.controlled_submit_transfer_v1(NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL)`)
		return classifyPrivilegeProbe(t, capability, err, map[string]bool{"22023": true})
	case "function.controlled_post_funding_v1.EXECUTE":
		_, err := database.ExecContext(ctx, `SELECT * FROM public.controlled_post_funding_v1(NULL,NULL,NULL,NULL,NULL,NULL)`)
		return classifyPrivilegeProbe(t, capability, err, map[string]bool{"22023": true})
	case "function.controlled_post_transfer_correction_v1.EXECUTE":
		_, err := database.ExecContext(ctx, `SELECT * FROM public.controlled_post_transfer_correction_v1(NULL,NULL,NULL,NULL,NULL,NULL,NULL)`)
		return classifyPrivilegeProbe(t, capability, err, map[string]bool{"22023": true})
	case "function.controlled_provision_account_v1.EXECUTE":
		_, err := database.ExecContext(ctx, `SELECT * FROM public.controlled_provision_account_v1(NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL)`)
		return classifyPrivilegeProbe(t, capability, err, map[string]bool{"22023": true})
	case "database.schema.CREATE":
		return probeSchemaCreate(t, database)
	}

	parts := strings.Split(capability, ".")
	if len(parts) != 3 || parts[0] != "table" {
		t.Fatalf("unsupported capability probe %q", capability)
	}
	table, privilege := parts[1], parts[2]
	quotedTable := pgx.Identifier{"public", table}.Sanitize()
	var statement string
	switch privilege {
	case "SELECT":
		statement = `SELECT * FROM ` + quotedTable + ` LIMIT 0`
	case "INSERT":
		statement = `INSERT INTO ` + quotedTable + ` DEFAULT VALUES`
	case "UPDATE":
		statement = `UPDATE ` + quotedTable + ` SET ` + pgx.Identifier{tableUpdateColumns[table]}.Sanitize() + `=DEFAULT WHERE false`
	case "DELETE":
		statement = `DELETE FROM ` + quotedTable + ` WHERE false`
	default:
		t.Fatalf("unsupported table privilege %q", privilege)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin %s probe: %v", capability, err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, statement)
	allowedStates := map[string]bool{}
	if privilege == "INSERT" {
		allowedStates = map[string]bool{"22000": true, "23502": true, "23503": true, "23505": true, "23514": true, "P0001": true}
	}
	return classifyPrivilegeProbe(t, capability, err, allowedStates)
}

func probeSchemaCreate(t *testing.T, database *sql.DB) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin schema-create probe: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	name := pgx.Identifier{"capability_probe_" + randomHex(t, 6)}.Sanitize()
	_, err = tx.ExecContext(ctx, `CREATE SCHEMA `+name)
	return classifyPrivilegeProbe(t, "database.schema.CREATE", err, nil)
}

func classifyPrivilegeProbe(t *testing.T, capability string, err error, acceptedAllowedStates map[string]bool) bool {
	t.Helper()
	if err == nil {
		return true
	}
	state := sqlState(err)
	if state == "42501" {
		return false
	}
	if acceptedAllowedStates[state] {
		return true
	}
	t.Errorf("%s probe failed for a non-privilege reason (SQLSTATE %s): %v", capability, state, err)
	return false
}

func sqlState(err error) string {
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		return state.SQLState()
	}
	return "unknown"
}

func newCapabilityEvidence(installation string, expectation capabilityExpectation, actual bool) capabilityEvidence {
	status := "aligned"
	if actual != expectation.CurrentAllowed {
		status = "unexpected_drift"
	} else if actual != expectation.TargetAllowed {
		status = "hardening_gap"
	}
	return capabilityEvidence{
		MatrixVersion: databaseCapabilityMatrixVersion, Installation: installation,
		Role: expectation.Role, Capability: expectation.Capability, ActualAllowed: actual,
		CurrentAllowed: expectation.CurrentAllowed, TargetAllowed: expectation.TargetAllowed, Status: status,
	}
}

func writeCapabilityReport(t *testing.T, path string, evidence []capabilityEvidence) {
	t.Helper()
	payload := struct {
		MatrixVersion string               `json:"matrix_version"`
		GeneratedAt   string               `json:"generated_at"`
		Evidence      []capabilityEvidence `json:"evidence"`
	}{MatrixVersion: databaseCapabilityMatrixVersion, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Evidence: evidence}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("encode capability evidence: %v", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write capability evidence: %v", err)
	}
}

func randomHex(t *testing.T, bytes int) string {
	t.Helper()
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("generate ephemeral capability credential: %v", err)
	}
	return hex.EncodeToString(buffer)
}

func redactSecret(err error, secret string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), secret, "[redacted]"))
}
