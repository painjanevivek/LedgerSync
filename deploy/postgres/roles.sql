-- Run as the database owner during environment provisioning. These NOLOGIN
-- roles are granted to separately authenticated workload users by the secret
-- manager/IaC layer; credentials never live in this repository.
DO $$ BEGIN
  CREATE ROLE ledgersync_migration_owner NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_api NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_worker NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_reconciliation NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_support_readonly NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_break_glass NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO ledgersync_api, ledgersync_worker, ledgersync_reconciliation, ledgersync_support_readonly;

GRANT SELECT ON tenants, accounts, account_owners, account_credit_permissions, account_balance_projections,
  tenant_transfer_policies, transfers, idempotency_requests, api_rate_limit_windows,
  journal_transactions, ledger_postings, delivery_attempts,
  reconciliation_runs, reconciliation_mismatches TO ledgersync_api;
GRANT INSERT ON transfers, idempotency_requests, journal_transactions, ledger_postings,
  outbox_events, audit_events, api_rate_limit_windows TO ledgersync_api;
GRANT UPDATE ON transfers, idempotency_requests, account_balance_projections,
  api_rate_limit_windows TO ledgersync_api;

GRANT SELECT, UPDATE ON outbox_events TO ledgersync_worker;
GRANT INSERT ON delivery_attempts, audit_events TO ledgersync_worker;
GRANT SELECT ON transfers, tenants TO ledgersync_worker;

GRANT SELECT ON tenants, accounts, account_opening_balances,
  account_balance_projections, transfers, journal_transactions, ledger_postings,
  schema_migrations TO ledgersync_reconciliation;
GRANT INSERT ON reconciliation_runs, reconciliation_mismatches, audit_events TO ledgersync_reconciliation;

GRANT SELECT ON tenants, accounts, account_owners, account_balance_projections,
  transfers, journal_transactions, ledger_postings, delivery_attempts,
  reconciliation_runs, reconciliation_mismatches, audit_events TO ledgersync_support_readonly;

-- Break-glass has no standing object grants. Incident-authorized grants must be
-- time bounded, ticket correlated, audited, and revoked by the platform owner.
