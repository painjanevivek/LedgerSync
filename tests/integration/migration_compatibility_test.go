package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	accountapp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	fundingapp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	reconciliationapp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	transferapp "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func TestMigrationsAreForwardCompatibleAndPreserveExistingReadContracts(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	var versions int
	if err := database.QueryRowContext(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 36 {
		t.Fatalf("migration versions=%d, want 36", versions)
	}
	for _, table := range []string{"accounts", "account_credit_permissions", "ledger_postings", "ledger_semantic_key_validation", "ledger_semantic_control_events", "outbox_events", "reconciliation_runs", "reconciliation_mismatches", "reconciliation_run_commands", "delivery_attempts", "webhook_delivery_jobs", "delivery_replay_actions", "tenant_transfer_policies", "transfer_policy_versions", "transfer_corrections", "tenant_funding_policies", "funding_events", "approval_records", "funding_velocity_events", "api_rate_limit_windows", "transfer_velocity_events", "transfer_velocity_totals", "account_opening_balances", "retention_runs", "outbox_replay_actions", "partner_provisioning_requests", "partner_credential_events", "operator_onboarding_preferences", "investigation_saved_views", "investigation_workspaces", "investigation_workspace_references", "bff_actor_assertion_replays", "webhook_endpoint_verification_jobs"} {
		var exists bool
		if err := database.QueryRowContext(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("required table %s is missing after migration", table)
		}
	}
	for _, index := range []string{"funding_events_approval_queue_idx", "transfer_corrections_approval_queue_idx", "developer_webhook_endpoints_tenant_status_updated_idx", "developer_webhook_endpoints_subscriptions_idx", "delivery_attempts_webhook_endpoint_recent_idx", "delivery_attempts_webhook_event_endpoint_idx", "reconciliation_mismatches_tenant_transfer_idx", "journal_transactions_tenant_funding_idx", "outbox_events_tenant_account_relation_idx", "outbox_events_tenant_transfer_relation_idx", "transfer_corrections_tenant_compensation_idx", "investigation_saved_views_owner_name_idx", "investigation_saved_views_owner_recent_idx", "investigation_workspaces_owner_recent_idx", "investigation_workspace_references_record_idx"} {
		var exists bool
		if err := database.QueryRowContext(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, index).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("required approval read-model index %s is missing after migration", index)
		}
	}
	var columns int
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (table_name, column_name) IN (
    ('accounts', 'tenant_id'),
    ('account_balance_projections', 'available_minor'),
    ('account_balance_projections', 'ledger_minor'),
    ('ledger_postings', 'direction'),
    ('outbox_events', 'aggregate_version'),
    ('outbox_events', 'aggregate_type'),
    ('outbox_events', 'aggregate_id'),
    ('accounts', 'version'),
    ('accounts', 'updated_at'),
    ('accounts', 'account_kind'),
    ('account_balance_projections', 'allow_negative'),
    ('journal_transactions', 'funding_event_id'),
	    ('outbox_events', 'funding_event_id'),
	    ('journal_transactions', 'source_type'),
	    ('journal_transactions', 'source_id'),
	    ('ledger_postings', 'tenant_id'),
    ('transfers', 'policy_version'),
    ('transfers', 'compensation_of_transfer_id'),
    ('tenant_transfer_policies', 'policy_version'),
	    ('tenant_transfer_policies', 'control_mode'),
	    ('tenant_transfer_policies', 'requires_step_up'),
	    ('tenant_transfer_policies', 'approval_ttl_minutes'),
	    ('reconciliation_mismatches', 'transfer_id')
	  )`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 23 {
		t.Fatalf("legacy and additive account contract columns=%d, want 23", columns)
	}
	var compositeUniqueKeys, validatedCompositeForeignKeys, hardenedHydrators, hardenedSemanticFunctions, controlledFinancialFunctions, semanticTriggers int
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*) FROM pg_constraint
WHERE conname IN ('journal_transactions_id_tenant_key','ledger_postings_id_tenant_key')
  AND contype='u' AND convalidated`).Scan(&compositeUniqueKeys); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*) FROM pg_constraint
WHERE conname IN ('journal_transfer_tenant_fk','journal_funding_tenant_fk','ledger_posting_journal_tenant_fk','ledger_posting_account_tenant_fk')
  AND contype='f' AND convalidated`).Scan(&validatedCompositeForeignKeys); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*)
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
WHERE namespace.nspname='public'
  AND procedure.proname IN ('hydrate_journal_semantic_keys','hydrate_posting_tenant_key')
  AND NOT procedure.prosecdef
  AND 'search_path=pg_catalog, public'=ANY(procedure.proconfig)
  AND NOT EXISTS (SELECT 1 FROM aclexplode(procedure.proacl) acl WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE')`).Scan(&hardenedHydrators); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*) FROM pg_trigger
WHERE tgname IN ('journal_transactions_semantic_shape','ledger_postings_semantic_shape','transfers_semantic_shape','funding_events_semantic_shape')
  AND tgconstraint<>0
  AND (SELECT condeferrable AND condeferred FROM pg_constraint WHERE oid=tgconstraint)`).Scan(&semanticTriggers); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*)
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
WHERE namespace.nspname='public'
  AND procedure.proname IN ('validate_ledger_semantic_shape','enforce_ledger_semantic_shape')
  AND procedure.prosecdef
  AND 'search_path=pg_catalog, public'=ANY(procedure.proconfig)
  AND NOT EXISTS (SELECT 1 FROM aclexplode(procedure.proacl) acl WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE')`).Scan(&hardenedSemanticFunctions); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(context.Background(), `
SELECT count(*)
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
JOIN pg_roles owner ON owner.oid=procedure.proowner
WHERE namespace.nspname='public'
  AND procedure.proname IN ('controlled_submit_transfer_v1','controlled_post_funding_v1','controlled_post_transfer_correction_v1','controlled_provision_account_v1')
  AND procedure.prosecdef
  AND owner.rolname='ledgersync_migration_owner'
  AND 'search_path=pg_catalog, public'=ANY(procedure.proconfig)
  AND NOT EXISTS (SELECT 1 FROM aclexplode(procedure.proacl) acl WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE')`).Scan(&controlledFinancialFunctions); err != nil {
		t.Fatal(err)
	}
	if compositeUniqueKeys != 2 || validatedCompositeForeignKeys != 4 || hardenedHydrators != 2 || hardenedSemanticFunctions != 2 || controlledFinancialFunctions != 4 || semanticTriggers != 4 {
		t.Fatalf("ledger validation controls unique=%d validated_fk=%d hardened_hydrators=%d hardened_semantic_functions=%d controlled_financial_functions=%d semantic_triggers=%d", compositeUniqueKeys, validatedCompositeForeignKeys, hardenedHydrators, hardenedSemanticFunctions, controlledFinancialFunctions, semanticTriggers)
	}
}

func TestMigrationThirteenUpgradesPhaseSevenDataWithoutFinancialRewrite(t *testing.T) {
	databaseURL := os.Getenv("LEDGERSYNC_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("LEDGERSYNC_TEST_DATABASE_URL is required for migration upgrade tests")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("ledgersync_upgrade_%d", time.Now().UnixNano())
	admin, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: databaseURL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if _, err := admin.Exec(`CREATE DATABASE ` + databaseName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1`, databaseName)
		_, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + databaseName)
	})
	parsed.Path = "/" + databaseName
	upgradeDatabase, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: parsed.String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = upgradeDatabase.Close() })
	_, sourceFile, _, _ := runtime.Caller(0)
	migrationDirectory := filepath.Join(filepath.Dir(sourceFile), "..", "..", "migrations")
	phaseSeven := fstest.MapFS{}
	entries, err := os.ReadDir(migrationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") || entry.Name() >= "000013_" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(migrationDirectory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		phaseSeven[entry.Name()] = &fstest.MapFile{Data: content, Mode: fs.FileMode(0o600)}
	}
	if err := db.ApplyPending(context.Background(), upgradeDatabase, db.MigrationConfig{Source: phaseSeven}); err != nil {
		t.Fatal(err)
	}
	legacyTenant := "00000000-0000-0000-0000-000000000801"
	legacyAccounts := []string{
		"00000000-0000-0000-0000-000000000802",
		"00000000-0000-0000-0000-000000000803",
		"00000000-0000-0000-0000-000000000804",
		"00000000-0000-0000-0000-000000000805",
	}
	legacyTransferID := "00000000-0000-0000-0000-000000000807"
	legacyJournalID := "00000000-0000-0000-0000-000000000808"
	createdAt := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	if _, err := upgradeDatabase.Exec(`INSERT INTO tenants(id,external_reference)VALUES($1,'legacy-upgrade')`, legacyTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(`
INSERT INTO accounts(id,tenant_id,currency,status,created_at,display_name,external_reference)VALUES
($1,$5,'INR','active',$6,'   ','   '),
($2,$5,'INR','active',$6,'Mixed reference upper','Mixed-Ref'),
($3,$5,'INR','active',$6,'Mixed reference lower',' mixed-ref '),
($4,$5,'INR','active',$6,$7,' ../invalid?? ')`, legacyAccounts[0], legacyAccounts[1], legacyAccounts[2], legacyAccounts[3], legacyTenant, createdAt, strings.Repeat("x", 121)); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(`
INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version,updated_at)VALUES
($1,725,725,9,$5),($2,10,10,2,$5),($3,20,20,3,$5),($4,30,30,4,$5)`, legacyAccounts[0], legacyAccounts[1], legacyAccounts[2], legacyAccounts[3], createdAt); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(`
INSERT INTO account_opening_balances(account_id,opening_ledger_minor,created_at)VALUES
($1,730,$5),($2,5,$5),($3,20,$5),($4,30,$5)`, legacyAccounts[0], legacyAccounts[1], legacyAccounts[2], legacyAccounts[3], createdAt); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role)VALUES($1,'upgrade-operator','operator')`, legacyTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(`INSERT INTO account_owners(tenant_id,account_id,subject_id,permission)VALUES($1,$2,'upgrade-operator','debit')`, legacyTenant, legacyAccounts[0]); err != nil {
		t.Fatal(err)
	}
	legacyLedger, err := upgradeDatabase.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacyLedger.Exec(`
INSERT INTO transfers(id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,journal_transaction_id,created_at,completed_at)
VALUES($1,$2,'upgrade-operator',$3,$4,5,'INR','posted',$5,$6,$6)`, legacyTransferID, legacyTenant, legacyAccounts[0], legacyAccounts[1], legacyJournalID, createdAt); err != nil {
		_ = legacyLedger.Rollback()
		t.Fatal(err)
	}
	if _, err = legacyLedger.Exec(`INSERT INTO journal_transactions(id,tenant_id,transfer_id,occurred_at) VALUES($1,$2,$3,$4)`, legacyJournalID, legacyTenant, legacyTransferID, createdAt); err != nil {
		_ = legacyLedger.Rollback()
		t.Fatal(err)
	}
	if _, err = legacyLedger.Exec(`
INSERT INTO ledger_postings(id,journal_transaction_id,account_id,direction,amount_minor,currency,occurred_at) VALUES
('00000000-0000-0000-0000-000000000809',$1,$2,'debit',5,'INR',$4),
('00000000-0000-0000-0000-000000000810',$1,$3,'credit',5,'INR',$4)`, legacyJournalID, legacyAccounts[0], legacyAccounts[1], createdAt); err != nil {
		_ = legacyLedger.Rollback()
		t.Fatal(err)
	}
	if err = legacyLedger.Commit(); err != nil {
		t.Fatalf("commit historical ledger shape: %v", err)
	}
	const syntheticJournalCount = 5_000
	seedProductionLikeHistoricalLedger(t, upgradeDatabase, legacyTenant, legacyAccounts[0], legacyAccounts[1], createdAt, syntheticJournalCount)
	migrationStarted := time.Now()
	if err := db.ApplyPending(context.Background(), upgradeDatabase, db.MigrationConfig{Source: os.DirFS(migrationDirectory)}); err != nil {
		t.Fatal(err)
	}
	migrationDuration := time.Since(migrationStarted)
	t.Logf("expanded %d historical journals and %d postings in %s", syntheticJournalCount+1, syntheticJournalCount*2+2, migrationDuration)
	if migrationDuration > 30*time.Second {
		t.Fatalf("production-like ledger expansion exceeded the 30s rehearsal budget: %s", migrationDuration)
	}
	var sourceType, sourceID, journalTenant string
	var tenantAwarePostings int
	if err := upgradeDatabase.QueryRow(`
SELECT journal.source_type,journal.source_id::text,journal.tenant_id::text,
       count(posting.id) FILTER (WHERE posting.tenant_id=journal.tenant_id)
FROM journal_transactions AS journal
JOIN ledger_postings AS posting ON posting.journal_transaction_id=journal.id
WHERE journal.id=$1
GROUP BY journal.id`, legacyJournalID).Scan(&sourceType, &sourceID, &journalTenant, &tenantAwarePostings); err != nil {
		t.Fatal(err)
	}
	if sourceType != "transfer" || sourceID != legacyTransferID || journalTenant != legacyTenant || tenantAwarePostings != 2 {
		t.Fatalf("historical semantic backfill type=%q source=%q tenant=%q postings=%d", sourceType, sourceID, journalTenant, tenantAwarePostings)
	}
	assertNoLedgerSemanticKeyMismatches(t, upgradeDatabase)
	var expandedJournals, expandedPostings int
	if err := upgradeDatabase.QueryRow(`SELECT journal_row_count,posting_row_count FROM ledger_semantic_key_validation`).Scan(&expandedJournals, &expandedPostings); err != nil {
		t.Fatal(err)
	}
	if expandedJournals != syntheticJournalCount+1 || expandedPostings != syntheticJournalCount*2+2 {
		t.Fatalf("expanded row coverage journals=%d postings=%d", expandedJournals, expandedPostings)
	}
	rolesSQL, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "..", "deploy", "postgres", "roles.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(string(rolesSQL)); err != nil {
		t.Fatalf("apply post-upgrade database roles: %v", err)
	}
	verifyDatabaseRoleCapabilities(
		t,
		upgradeDatabase,
		parsed.String(),
		"supported-upgrade",
		"00000000-0000-4000-8000-000000000806",
		legacyTenant,
	)
	var canReadOutbox, canReadAudit, canReadFundingPolicy, canMutateFundingPolicy, canPersistAssertionReplay, canDeleteAssertionReplay, canInsertVerificationJob, workerCanClaimVerificationJob bool
	if err := upgradeDatabase.QueryRow(`
SELECT has_table_privilege('ledgersync_api','outbox_events','SELECT'),
       has_table_privilege('ledgersync_api','audit_events','SELECT'),
       has_table_privilege('ledgersync_api','tenant_funding_policies','SELECT'),
       has_table_privilege('ledgersync_api','tenant_funding_policies','UPDATE'),
       has_table_privilege('ledgersync_api','bff_actor_assertion_replays','INSERT'),
       has_table_privilege('ledgersync_api','bff_actor_assertion_replays','DELETE'),
       has_table_privilege('ledgersync_api','webhook_endpoint_verification_jobs','INSERT'),
       has_table_privilege('ledgersync_worker','webhook_endpoint_verification_jobs','SELECT')
         AND has_table_privilege('ledgersync_worker','webhook_endpoint_verification_jobs','UPDATE')`).Scan(&canReadOutbox, &canReadAudit, &canReadFundingPolicy, &canMutateFundingPolicy, &canPersistAssertionReplay, &canDeleteAssertionReplay, &canInsertVerificationJob, &workerCanClaimVerificationJob); err != nil {
		t.Fatal(err)
	}
	if !canReadOutbox || !canReadAudit || !canReadFundingPolicy || canMutateFundingPolicy || !canPersistAssertionReplay || !canDeleteAssertionReplay || !canInsertVerificationJob || !workerCanClaimVerificationJob {
		t.Fatalf("migrations did not preserve least-privilege workload grants: outbox=%t audit=%t funding_policy_read=%t funding_policy_update=%t assertion_insert=%t assertion_delete=%t verification_insert=%t worker_verification_claim=%t", canReadOutbox, canReadAudit, canReadFundingPolicy, canMutateFundingPolicy, canPersistAssertionReplay, canDeleteAssertionReplay, canInsertVerificationJob, workerCanClaimVerificationJob)
	}
	wantBalances := map[string][3]int64{
		legacyAccounts[0]: {725, 725, 9}, legacyAccounts[1]: {10, 10, 2}, legacyAccounts[2]: {20, 20, 3}, legacyAccounts[3]: {30, 30, 4},
	}
	rows, err := upgradeDatabase.Query(`
SELECT a.id,a.tenant_id,a.currency,a.display_name,a.external_reference,a.version,a.created_at,a.updated_at,
       b.available_minor,b.ledger_minor,b.balance_version
FROM accounts a JOIN account_balance_projections b ON b.account_id=a.id
WHERE a.tenant_id=$1 ORDER BY a.id`, legacyTenant)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	seenReferences := map[string]bool{}
	for rows.Next() {
		var id, tenant, currency, displayName, reference string
		var available, ledger, balanceVersion, accountVersion int64
		var storedCreatedAt, updatedAt time.Time
		if err := rows.Scan(&id, &tenant, &currency, &displayName, &reference, &accountVersion, &storedCreatedAt, &updatedAt, &available, &ledger, &balanceVersion); err != nil {
			t.Fatal(err)
		}
		want, ok := wantBalances[id]
		if !ok || tenant != legacyTenant || currency != "INR" || available != want[0] || ledger != want[1] || balanceVersion != want[2] || accountVersion != 1 || !storedCreatedAt.Equal(createdAt) || !updatedAt.Equal(createdAt) || displayName == "" || len([]rune(displayName)) > 120 || reference == "" {
			t.Fatalf("migration rewrote legacy financial identity/state: id=%s tenant=%s currency=%s available=%d ledger=%d balanceVersion=%d accountVersion=%d created=%s updated=%s name=%q reference=%q", id, tenant, currency, available, ledger, balanceVersion, accountVersion, storedCreatedAt, updatedAt, displayName, reference)
		}
		normalizedReference := strings.ToLower(reference)
		if seenReferences[normalizedReference] {
			t.Fatalf("migration left duplicate normalized reference %q", normalizedReference)
		}
		seenReferences[normalizedReference] = true
		delete(wantBalances, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(wantBalances) != 0 || len(seenReferences) != 4 {
		t.Fatalf("migration account coverage missing=%v references=%v", wantBalances, seenReferences)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	limitedDatabase := provisionWorkloadSession(t, upgradeDatabase, parsed.String(), "ledgersync_api").db
	commandRepository, err := db.NewAccountCommandRepository(limitedDatabase, func() time.Time { return createdAt.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	commandService, err := accountapp.NewCommandService(commandRepository, func() time.Time { return createdAt.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	created, err := commandService.Create(context.Background(), accountapp.CreateAccountCommand{
		TenantID: legacyTenant, ActorSubjectID: "upgrade-operator", CorrelationID: "00000000-0000-0000-0000-000000000899",
		IdempotencyKey: "upgrade-role-create", DisplayName: "Upgrade-created account", Reference: "upgrade-created", Category: "operating", Currency: "INR",
	})
	if err != nil {
		_ = limitedDatabase.Close()
		t.Fatalf("account command with migrated API grants: %v", err)
	}
	reconciliationRepository, err := db.NewReconciliationRepository(limitedDatabase)
	if err != nil {
		t.Fatal(err)
	}
	reconciliationService, err := reconciliationapp.NewCommandService(reconciliationRepository, func() time.Time { return createdAt.Add(2 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := reconciliationService.Run(context.Background(), reconciliationapp.RunCommand{
		TenantID: legacyTenant, ActorSubjectID: "upgrade-operator", CorrelationID: "00000000-0000-0000-0000-000000000898", IdempotencyKey: "upgrade-role-reconciliation-run",
	})
	if err != nil || reconciled.Result.ID == "" {
		_ = limitedDatabase.Close()
		t.Fatalf("reconciliation command with migrated API grants: result=%#v error=%v", reconciled, err)
	}
	if created.Result.AvailableMinor != "0" || created.Result.LedgerMinor != "0" || countRowsInDatabase(t, upgradeDatabase, `SELECT count(*) FROM accounts WHERE id=$1`, created.Result.AccountID) != 1 {
		t.Fatalf("limited-role account create result=%#v", created.Result)
	}
	if _, err := upgradeDatabase.Exec(`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,'upgrade-operator','finance')`, legacyTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeDatabase.Exec(`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'INR','local_demo_single_operator',false,1,100000,200000,500000)`, legacyTenant); err != nil {
		t.Fatal(err)
	}
	fundingRepository, err := db.NewFundingRepository(limitedDatabase, func() time.Time { return createdAt.Add(3 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	fundingService, err := fundingapp.NewService(fundingRepository, fundingapp.PolicyLocalDemoSingleOperator, func() time.Time { return createdAt.Add(3 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	fundingAmount, err := money.New("INR", 125)
	if err != nil {
		t.Fatal(err)
	}
	fundingRequest, err := fundingService.Request(context.Background(), fundingapp.RequestCommand{
		TenantID: legacyTenant, ActorSubjectID: "upgrade-operator", DestinationAccountID: created.Result.AccountID, Amount: fundingAmount,
		ExternalReference: "upgrade-funding-evidence", EvidenceReference: "customer-evidence://upgrade/funding",
		IdempotencyKey: "upgrade-role-funding-0001", CorrelationID: "00000000-0000-0000-0000-000000000897",
	})
	if err != nil {
		t.Fatalf("funding request with fresh migrated system account: %v", err)
	}
	approvedFunding, err := fundingService.Approve(context.Background(), fundingapp.DecisionCommand{
		TenantID: legacyTenant, ActorSubjectID: "upgrade-operator", FundingEventID: fundingRequest.Event.FundingEventID,
		Reason: "verified upgrade evidence", CorrelationID: "00000000-0000-0000-0000-000000000896",
	})
	if err != nil || approvedFunding.Status != "approved" {
		t.Fatalf("funding approval after upgrade=%#v error=%v", approvedFunding, err)
	}
	postedFunding, err := fundingService.Post(context.Background(), fundingapp.ActionCommand{
		TenantID: legacyTenant, ActorSubjectID: "upgrade-operator", FundingEventID: fundingRequest.Event.FundingEventID,
		IdempotencyKey: "migration-funding-post-0001", CorrelationID: "00000000-0000-0000-0000-000000000895",
	})
	if err != nil || postedFunding.Event.Status != "posted" ||
		countRowsInDatabase(t, upgradeDatabase, `SELECT count(*) FROM accounts WHERE tenant_id=$1 AND account_kind='funding_clearing' AND category='system'`, legacyTenant) != 1 {
		t.Fatalf("fresh migrated funding journal=%#v error=%v", postedFunding, err)
	}
	if err := seedTransferFixture(context.Background(), upgradeDatabase, 10_000); err != nil {
		t.Fatalf("seed limited-role transfer fixture: %v", err)
	}
	transferRepository, err := db.NewTransferRepository(limitedDatabase, func() time.Time { return createdAt.Add(4 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	transferService, err := transferapp.NewService(transferRepository, func() time.Time { return createdAt.Add(4 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	firstTransfer, err := transferService.Submit(context.Background(), transferCommand(t, "upgrade-role-transfer-0001", "25.00"))
	if err != nil {
		t.Fatalf("atomic transfer with migrated API grants: %v", err)
	}
	replayedTransfer, err := transferService.Submit(context.Background(), transferCommand(t, "upgrade-role-transfer-0001", "25.00"))
	if err != nil {
		t.Fatalf("atomic transfer replay with migrated API grants: %v", err)
	}
	if firstTransfer.Replayed || !replayedTransfer.Replayed || firstTransfer.Result.TransferID == "" || firstTransfer.Result.TransferID != replayedTransfer.Result.TransferID ||
		countRowsInDatabase(t, upgradeDatabase, `SELECT count(*) FROM transfers WHERE id=$1 AND status='posted'`, firstTransfer.Result.TransferID) != 1 ||
		countRowsInDatabase(t, upgradeDatabase, `SELECT count(*) FROM journal_transactions WHERE transfer_id=$1`, firstTransfer.Result.TransferID) != 1 ||
		countRowsInDatabase(t, upgradeDatabase, `SELECT count(*) FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.transfer_id=$1`, firstTransfer.Result.TransferID) != 2 {
		t.Fatalf("limited-role transfer/replay did not commit exactly one balanced movement: first=%#v replay=%#v", firstTransfer, replayedTransfer)
	}
	if err := limitedDatabase.Close(); err != nil {
		t.Fatal(err)
	}
}

func countRowsInDatabase(t *testing.T, database interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, query string, args ...any) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
