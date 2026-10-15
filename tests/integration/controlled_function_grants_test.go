package integration_test

import (
	"strings"
	"testing"
)

// Catalog ACL checks complement positive workflow tests. They cover every
// controlled signature, including helpers that workloads must never call.
func TestControlledFunctionExecuteAllowlist(t *testing.T) {
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	allowed := map[string]string{
		"controlled_request_opening_import_v1":            "ledgersync_provisioning",
		"controlled_approve_opening_import_v1":            "ledgersync_provisioning",
		"controlled_execute_opening_import_v1":            "ledgersync_provisioning",
		"controlled_retire_bff_sessions_v1":               "ledgersync_api",
		"controlled_submit_transfer_v1":                   "ledgersync_api",
		"controlled_post_funding_v1":                      "ledgersync_api",
		"controlled_post_transfer_correction_v1":          "ledgersync_api",
		"controlled_provision_account_v1":                 "ledgersync_api ledgersync_provisioning",
		"controlled_prepare_transfer_recovery_v1":         "ledgersync_api",
		"controlled_attach_transfer_request_reference_v1": "ledgersync_api",
		"controlled_claim_outbox_v1":                      "ledgersync_worker",
		"controlled_claim_webhook_delivery_v1":            "ledgersync_worker",
		"controlled_claim_webhook_verification_v1":        "ledgersync_worker",
		"controlled_claim_tenant_v1":                      "ledgersync_worker",
		"controlled_pilot_currency_valid_v1":              "ledgersync_api ledgersync_worker ledgersync_reconciliation ledgersync_scheduler ledgersync_provisioning",
		"controlled_complete_webhook_verification_v1":     "ledgersync_worker",
		"controlled_ensure_funding_account_v1":            "ledgersync_api",
		"controlled_lock_owned_account_v1":                "ledgersync_api",
		"controlled_update_account_v1":                    "ledgersync_api",
		"controlled_lock_funding_policy_v1":               "ledgersync_api",
		"controlled_lock_correction_original_v1":          "ledgersync_api",
		"controlled_apply_partner_v1":                     "ledgersync_provisioning",
		"controlled_rollback_partner_v1":                  "ledgersync_provisioning",
		"controlled_append_audit_v1":                      "ledgersync_api ledgersync_worker ledgersync_reconciliation ledgersync_scheduler",
		"controlled_fingerprint_parts_v1":                 "",
		"controlled_request_funding_v1":                   "ledgersync_api",
		"controlled_request_funding_compensation_v1":      "ledgersync_api",
		"controlled_decide_funding_v1":                    "ledgersync_api",
		"controlled_request_correction_v1":                "ledgersync_api",
		"controlled_decide_correction_v1":                 "ledgersync_api",
		"controlled_renew_claim_v1":                       "ledgersync_worker",
		"controlled_complete_outbox_v1":                   "ledgersync_worker",
		"controlled_complete_delivery_v1":                 "ledgersync_worker",
		"controlled_prepare_publication_v1":               "ledgersync_worker",
		"controlled_receive_message_v1":                   "ledgersync_worker",
		"controlled_outbox_recovery_v1":                   "ledgersync_api",
		"controlled_delivery_recovery_v1":                 "ledgersync_api",
		"controlled_record_reconciliation_v1":             "ledgersync_api ledgersync_reconciliation ledgersync_scheduler",
		"controlled_run_retention_v1":                     "ledgersync_maintenance",
		"controlled_import_delivery_evidence_v1":          "",
	}
	rows, err := admin.Query(`SELECT p.oid,p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname LIKE 'controlled_%'`)
	if err != nil {
		t.Fatal(err)
	}
	type routine struct {
		oid  uint32
		name string
	}
	var routines []routine
	for rows.Next() {
		var item routine
		if err := rows.Scan(&item.oid, &item.name); err != nil {
			t.Fatal(err)
		}
		routines = append(routines, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range routines {
		roles, exists := allowed[item.name]
		if !exists {
			t.Errorf("controlled function absent from allowlist: %s", item.name)
			continue
		}
		if seen[item.name] {
			t.Errorf("unreviewed function overload: %s", item.name)
		}
		seen[item.name] = true
		for _, role := range append(append([]string{}, capabilityRoles...), "ledgersync_scheduler") {
			var actual bool
			if err := admin.QueryRow(`SELECT has_function_privilege($1,$2::oid,'EXECUTE')`, role, item.oid).Scan(&actual); err != nil {
				t.Fatal(err)
			}
			expected := strings.Contains(" "+roles+" ", " "+role+" ")
			if actual != expected {
				t.Errorf("%s EXECUTE %s=%t want %t", role, item.name, actual, expected)
			}
		}
	}
	if len(seen) != len(allowed) {
		t.Fatalf("controlled function inventory got=%d want=%d", len(seen), len(allowed))
	}
}
