-- Run as the database owner during environment provisioning. These NOLOGIN
-- roles are granted to separately authenticated workload users by the secret
-- manager/IaC layer; credentials never live in this repository.
DO $$ BEGIN
  CREATE ROLE ledgersync_migration_owner NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_api NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_worker NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_reconciliation NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_provisioning NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_support_readonly NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_break_glass NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO ledgersync_api, ledgersync_worker, ledgersync_reconciliation, ledgersync_provisioning, ledgersync_support_readonly;

GRANT SELECT ON tenants, accounts, account_owners, account_credit_permissions, account_balance_projections, account_opening_balances,
  tenant_transfer_policies, tenant_subject_roles, partner_credential_events, developer_credentials, developer_credential_events, developer_command_idempotency,
  developer_webhook_endpoints, developer_webhook_events, developer_webhook_command_idempotency, transfers, idempotency_requests, api_rate_limit_windows,
  bff_actor_assertion_replays,
  transfer_velocity_events, transfer_velocity_totals,
  journal_transactions, ledger_postings, delivery_attempts, webhook_delivery_jobs, delivery_replay_actions,
  reconciliation_runs, reconciliation_mismatches, outbox_events, audit_events, schema_migrations TO ledgersync_api;
GRANT INSERT ON accounts, account_balance_projections, account_opening_balances, account_owners, account_credit_permissions,
  transfers, idempotency_requests, journal_transactions, ledger_postings,
  reconciliation_runs, reconciliation_mismatches,
  outbox_events, audit_events, api_rate_limit_windows, retention_runs,
  transfer_velocity_events, transfer_velocity_totals,
  outbox_replay_actions, delivery_replay_actions, developer_credentials, developer_credential_events, developer_command_idempotency,
  developer_webhook_endpoints, developer_webhook_events, developer_webhook_command_idempotency, webhook_delivery_jobs,
  webhook_endpoint_verification_jobs, bff_actor_assertion_replays TO ledgersync_api;
GRANT UPDATE ON accounts, transfers, idempotency_requests, account_balance_projections,
  api_rate_limit_windows, transfer_velocity_totals, developer_credentials, developer_command_idempotency,
  developer_webhook_endpoints, developer_webhook_command_idempotency TO ledgersync_api;
GRANT DELETE ON transfer_velocity_events, bff_actor_assertion_replays TO ledgersync_api;

DO $$
BEGIN
  IF to_regclass('public.reconciliation_run_commands') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,DELETE ON reconciliation_run_commands TO ledgersync_api';
  END IF;
END $$;

DO $$
BEGIN
  IF to_regclass('public.operator_onboarding_preferences') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON operator_onboarding_preferences TO ledgersync_api';
  END IF;
  IF to_regclass('public.investigation_saved_views') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE,DELETE ON investigation_saved_views TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON investigation_saved_views TO ledgersync_support_readonly';
  END IF;
  IF to_regclass('public.investigation_workspaces') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON investigation_workspaces TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_workspace_references TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON investigation_workspaces,investigation_workspace_references TO ledgersync_support_readonly';
  END IF;
END $$;

DO $$
BEGIN
  IF to_regclass('public.funding_events') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON funding_events TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON approval_records,funding_velocity_events TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON tenant_funding_policies TO ledgersync_api';
  END IF;
  IF to_regclass('public.transfer_corrections') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON transfer_corrections TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON transfer_policy_versions TO ledgersync_api';
  END IF;
END $$;

GRANT SELECT, UPDATE ON outbox_events, webhook_delivery_jobs, webhook_endpoint_verification_jobs TO ledgersync_worker;
GRANT INSERT ON delivery_attempts, audit_events, outbox_replay_actions, delivery_replay_actions TO ledgersync_worker;
GRANT SELECT ON transfers, tenants, outbox_replay_actions, delivery_attempts, delivery_replay_actions,
  developer_webhook_endpoints, webhook_delivery_jobs TO ledgersync_worker;

GRANT SELECT ON tenants, accounts, account_opening_balances,
  account_balance_projections, transfers, journal_transactions, ledger_postings,
  schema_migrations TO ledgersync_reconciliation;
GRANT INSERT ON reconciliation_runs, reconciliation_mismatches, audit_events TO ledgersync_reconciliation;

GRANT SELECT ON tenants, transfers, partner_provisioning_requests, partner_credential_events TO ledgersync_provisioning;
GRANT INSERT ON tenants, tenant_transfer_policies, tenant_subject_roles, partner_credential_events,
  accounts, account_balance_projections, account_opening_balances, account_owners,
  account_credit_permissions, partner_provisioning_requests, audit_events TO ledgersync_provisioning;
GRANT UPDATE ON accounts TO ledgersync_provisioning;
GRANT DELETE ON account_credit_permissions, account_owners, tenant_subject_roles TO ledgersync_provisioning;

GRANT SELECT ON tenants, accounts, account_owners, account_balance_projections,
  transfers, journal_transactions, ledger_postings, delivery_attempts, webhook_delivery_jobs,
  reconciliation_runs, reconciliation_mismatches, audit_events TO ledgersync_support_readonly;
GRANT SELECT ON retention_runs, outbox_replay_actions, delivery_replay_actions, partner_provisioning_requests,
  tenant_subject_roles, partner_credential_events, developer_credentials, developer_credential_events,
  developer_webhook_endpoints, developer_webhook_events TO ledgersync_support_readonly;
GRANT SELECT (id,tenant_id,webhook_id,expires_at,attempt_number,status,available_at,last_error_code,completed_at,created_at,updated_at)
  ON webhook_endpoint_verification_jobs TO ledgersync_support_readonly;

-- Break-glass has no standing object grants. Incident-authorized grants must be
-- time bounded, ticket correlated, audited, and revoked by the platform owner.
