-- Run as the database owner during environment provisioning. These NOLOGIN
-- roles are granted to separately authenticated workload users by the secret
-- manager/IaC layer; credentials never live in this repository.
DO $$ BEGIN
  CREATE ROLE ledgersync_migration_owner NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_api NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_worker NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_reconciliation NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_scheduler NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_provisioning NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_support_readonly NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_break_glass NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE ROLE ledgersync_maintenance NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Existing role names are not proof of safe attributes or inheritance. Stop
-- provisioning on a hostile/pre-existing binding instead of hiding the drift.
DO $$ DECLARE candidate record; BEGIN
 FOR candidate IN SELECT oid,rolname,rolsuper,rolcanlogin,rolcreaterole,rolcreatedb,rolreplication,rolbypassrls
 FROM pg_roles WHERE rolname IN ('ledgersync_migration_owner','ledgersync_api','ledgersync_worker',
 'ledgersync_reconciliation','ledgersync_scheduler','ledgersync_provisioning','ledgersync_support_readonly',
 'ledgersync_break_glass','ledgersync_queue_owner','ledgersync_preflight_owner','ledgersync_maintenance','ledgersync_maintenance_owner') LOOP
  IF candidate.rolsuper OR candidate.rolcanlogin OR candidate.rolcreaterole OR candidate.rolcreatedb OR candidate.rolreplication OR candidate.rolbypassrls
   OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=candidate.oid) THEN
   RAISE EXCEPTION 'unsafe LedgerSync role attributes or inheritance: %',candidate.rolname USING ERRCODE='42501';
  END IF;
  IF candidate.rolname IN ('ledgersync_migration_owner','ledgersync_queue_owner','ledgersync_preflight_owner','ledgersync_maintenance_owner')
   AND EXISTS(SELECT 1 FROM pg_auth_members WHERE roleid=candidate.oid) THEN
   RAISE EXCEPTION 'function owner must not have members: %',candidate.rolname USING ERRCODE='42501';
  END IF;
 END LOOP;
END $$;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO ledgersync_migration_owner, ledgersync_api, ledgersync_worker, ledgersync_reconciliation, ledgersync_provisioning, ledgersync_support_readonly;

DO $$
BEGIN
  IF to_regprocedure('public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)') IS NOT NULL THEN
    EXECUTE 'ALTER FUNCTION public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid) OWNER TO ledgersync_migration_owner';
    EXECUTE 'REVOKE ALL ON FUNCTION public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid) FROM PUBLIC';
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid) TO ledgersync_api';
  END IF;
  IF to_regprocedure('public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid,text,timestamptz)') IS NOT NULL THEN
    EXECUTE 'ALTER FUNCTION public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid,text,timestamptz) OWNER TO ledgersync_migration_owner';
    EXECUTE 'REVOKE ALL ON FUNCTION public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid,text,timestamptz) FROM PUBLIC';
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid,text,timestamptz) TO ledgersync_api';
  END IF;
  IF to_regprocedure('public.controlled_post_funding_v1(uuid,text,uuid,text,uuid,timestamptz)') IS NOT NULL THEN
    EXECUTE 'ALTER FUNCTION public.controlled_post_funding_v1(uuid,text,uuid,text,uuid,timestamptz) OWNER TO ledgersync_migration_owner';
    EXECUTE 'REVOKE ALL ON FUNCTION public.controlled_post_funding_v1(uuid,text,uuid,text,uuid,timestamptz) FROM PUBLIC';
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_post_funding_v1(uuid,text,uuid,text,uuid,timestamptz) TO ledgersync_api';
  END IF;
  IF to_regprocedure('public.controlled_post_transfer_correction_v1(uuid,text,uuid,text,uuid,timestamptz,timestamptz)') IS NOT NULL THEN
    EXECUTE 'ALTER FUNCTION public.controlled_post_transfer_correction_v1(uuid,text,uuid,text,uuid,timestamptz,timestamptz) OWNER TO ledgersync_migration_owner';
    EXECUTE 'REVOKE ALL ON FUNCTION public.controlled_post_transfer_correction_v1(uuid,text,uuid,text,uuid,timestamptz,timestamptz) FROM PUBLIC';
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_post_transfer_correction_v1(uuid,text,uuid,text,uuid,timestamptz,timestamptz) TO ledgersync_api';
  END IF;
  IF to_regprocedure('public.controlled_provision_account_v1(uuid,text,uuid,text,text,text,text,text[],text[],text[],uuid,timestamptz)') IS NOT NULL THEN
    EXECUTE 'ALTER FUNCTION public.controlled_provision_account_v1(uuid,text,uuid,text,text,text,text,text[],text[],text[],uuid,timestamptz) OWNER TO ledgersync_migration_owner';
    EXECUTE 'REVOKE ALL ON FUNCTION public.controlled_provision_account_v1(uuid,text,uuid,text,text,text,text,text[],text[],text[],uuid,timestamptz) FROM PUBLIC';
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_provision_account_v1(uuid,text,uuid,text,text,text,text,text[],text[],text[],uuid,timestamptz) TO ledgersync_api,ledgersync_provisioning';
  END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_complete_webhook_verification_v1(uuid,text,bigint,text,timestamptz,timestamptz,text,text)') IS NOT NULL THEN
  GRANT UPDATE(status,version,challenge_digest,challenge_expires_at,verified_at,updated_at) ON developer_webhook_endpoints TO ledgersync_queue_owner;
  GRANT INSERT ON developer_webhook_events TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_complete_webhook_verification_v1(uuid,text,bigint,text,timestamptz,timestamptz,text,text) OWNER TO ledgersync_queue_owner;
  REVOKE ALL ON FUNCTION controlled_complete_webhook_verification_v1(uuid,text,bigint,text,timestamptz,timestamptz,text,text) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_complete_webhook_verification_v1(uuid,text,bigint,text,timestamptz,timestamptz,text,text) TO ledgersync_worker;
 END IF;
END $$;

-- SECURITY DEFINER command functions run as the non-login migration owner.
-- Trigger invocation comes from its table trigger. Workloads cannot invoke or
-- bind internal trigger helpers themselves. New functions require explicit ACLs.
DO $$ DECLARE helper record; BEGIN
 FOR helper IN SELECT p.oid::regprocedure AS signature FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname='public' AND p.prorettype='trigger'::regtype LOOP
  EXECUTE format('REVOKE EXECUTE ON FUNCTION %s FROM PUBLIC',helper.signature);
 END LOOP;
END $$;
-- Per-schema REVOKE cannot remove PostgreSQL's global default PUBLIC grant.
ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE ledgersync_migration_owner REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

DO $$ BEGIN
 IF to_regclass('public.transfer_recovery_intents') IS NOT NULL THEN
   GRANT SELECT,INSERT ON transfer_recovery_intents TO ledgersync_migration_owner;
   GRANT SELECT ON transfer_recovery_intents TO ledgersync_api;
   ALTER FUNCTION controlled_prepare_transfer_recovery_v1(UUID,TEXT,UUID,TEXT,UUID,UUID,TEXT,BIGINT,BYTEA) OWNER TO ledgersync_migration_owner;
   REVOKE ALL ON FUNCTION controlled_prepare_transfer_recovery_v1(UUID,TEXT,UUID,TEXT,UUID,UUID,TEXT,BIGINT,BYTEA) FROM PUBLIC;
   GRANT EXECUTE ON FUNCTION controlled_prepare_transfer_recovery_v1(UUID,TEXT,UUID,TEXT,UUID,UUID,TEXT,BIGINT,BYTEA) TO ledgersync_api;
 END IF;
END $$;

-- Apply its least-privilege object ACL after ownership transfer and before any
-- workload execution. Workloads receive EXECUTE, never membership in this role.
GRANT SELECT ON tenants, tenant_subject_roles, tenant_transfer_policies, accounts,
  account_owners, account_credit_permissions, account_balance_projections,
  account_opening_balances,
  idempotency_requests, transfer_velocity_events, transfer_velocity_totals,
  transfers, journal_transactions, ledger_postings, developer_webhook_endpoints,
  tenant_funding_policies, funding_events, approval_records, funding_velocity_events,
  transfer_corrections
  TO ledgersync_migration_owner;
GRANT INSERT ON idempotency_requests, transfer_velocity_events, transfer_velocity_totals,
  transfers, journal_transactions, ledger_postings, audit_events, outbox_events,
  webhook_delivery_jobs, funding_velocity_events, approval_records, accounts,
  account_balance_projections, account_opening_balances, account_owners,
  account_credit_permissions TO ledgersync_migration_owner;
-- PostgreSQL requires UPDATE privilege for SELECT ... FOR SHARE even when the
-- controlled function never mutates the policy row.
GRANT UPDATE ON tenant_transfer_policies, accounts, idempotency_requests,
  transfer_velocity_totals, transfers, account_balance_projections, funding_events,
  transfer_corrections
  TO ledgersync_migration_owner;
GRANT DELETE ON transfer_velocity_events TO ledgersync_migration_owner;

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
  IF to_regclass('public.bff_sessions') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE,DELETE ON bff_sessions TO ledgersync_api';
  END IF;
  IF to_regclass('public.operator_ui_preferences') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON operator_ui_preferences TO ledgersync_api';
  END IF;
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
  IF to_regclass('public.investigation_live_rooms') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON investigation_live_rooms TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_live_room_findings,investigation_live_room_operations TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON investigation_live_rooms,investigation_live_room_findings TO ledgersync_support_readonly';
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
GRANT SELECT ON outbox_events TO ledgersync_reconciliation;
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
-- Publication attempts are delivery evidence and have no financial authority.
GRANT SELECT, INSERT, UPDATE ON outbox_publication_attempts TO ledgersync_worker;
GRANT SELECT ON outbox_publication_attempts TO ledgersync_api;
GRANT SELECT ON logical_node_clocks,logical_message_receipts TO ledgersync_api;
GRANT SELECT,INSERT ON logical_message_receipts TO ledgersync_worker;
GRANT SELECT ON logical_node_clocks TO ledgersync_worker;
GRANT EXECUTE ON FUNCTION advance_logical_clock(text,bigint) TO ledgersync_api,ledgersync_worker;
GRANT USAGE ON SCHEMA public,ledgersync_scheduler TO ledgersync_scheduler;
GRANT SELECT ON ALL TABLES IN SCHEMA ledgersync_scheduler TO ledgersync_scheduler;
GRANT INSERT ON reconciliation_runs,reconciliation_mismatches,audit_events TO ledgersync_scheduler;
GRANT EXECUTE ON FUNCTION scheduler_tenant_allowed(uuid),acquire_reconciliation_scheduler(uuid,text,integer),renew_reconciliation_scheduler(uuid,text,bigint,integer),release_reconciliation_scheduler(uuid,text,bigint),claim_reconciliation_schedule(uuid,text,bigint,integer),complete_reconciliation_schedule(uuid,text,bigint,uuid,uuid,text) TO ledgersync_scheduler;
GRANT SELECT ON reconciliation_scheduler_leases,reconciliation_scheduled_jobs TO ledgersync_api;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_ensure_funding_account_v1(uuid,text,text,uuid,timestamptz)') IS NOT NULL THEN
  ALTER FUNCTION controlled_ensure_funding_account_v1(uuid,text,text,uuid,timestamptz) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_ensure_funding_account_v1(uuid,text,text,uuid,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_ensure_funding_account_v1(uuid,text,text,uuid,timestamptz) TO ledgersync_api;
  REVOKE INSERT ON accounts,account_opening_balances,account_balance_projections,account_owners,account_credit_permissions FROM ledgersync_api,ledgersync_provisioning;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.tenant_scope_account(uuid)') IS NOT NULL THEN
  ALTER FUNCTION tenant_scope_account(uuid) OWNER TO ledgersync_migration_owner;
  GRANT USAGE ON SCHEMA public TO ledgersync_preflight_owner;
  GRANT SELECT ON tenants,tenant_transfer_policies,accounts,transfers TO ledgersync_preflight_owner;
  ALTER FUNCTION controlled_pilot_currency_valid_v1(text) OWNER TO ledgersync_preflight_owner;
  REVOKE ALL ON FUNCTION controlled_pilot_currency_valid_v1(text) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_pilot_currency_valid_v1(text) TO ledgersync_api,ledgersync_worker,ledgersync_reconciliation,ledgersync_scheduler,ledgersync_provisioning;
 END IF;
END $$;

-- Queue discovery is available only through bounded, fenced functions. This
-- owner has no financial table grants and is never granted to a workload.
DO $$ BEGIN
 IF to_regprocedure('public.controlled_claim_tenant_v1(text,uuid,text,bigint)') IS NOT NULL THEN
  GRANT USAGE ON SCHEMA public TO ledgersync_queue_owner;
  GRANT SELECT,UPDATE ON outbox_events,webhook_delivery_jobs,webhook_endpoint_verification_jobs TO ledgersync_queue_owner;
  GRANT SELECT ON developer_webhook_endpoints TO ledgersync_queue_owner;
  GRANT SELECT,INSERT ON outbox_publication_attempts TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_claim_outbox_v1(integer,text,double precision) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_claim_webhook_delivery_v1(double precision,integer,text) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_claim_webhook_verification_v1(double precision,integer,text) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_claim_tenant_v1(text,uuid,text,bigint) OWNER TO ledgersync_queue_owner;
  REVOKE ALL ON FUNCTION controlled_claim_outbox_v1(integer,text,double precision),controlled_claim_webhook_delivery_v1(double precision,integer,text),controlled_claim_webhook_verification_v1(double precision,integer,text),controlled_claim_tenant_v1(text,uuid,text,bigint) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_claim_outbox_v1(integer,text,double precision),controlled_claim_webhook_delivery_v1(double precision,integer,text),controlled_claim_webhook_verification_v1(double precision,integer,text),controlled_claim_tenant_v1(text,uuid,text,bigint) TO ledgersync_worker;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_update_account_v1(uuid,text,uuid,bigint,text,text,text,text,uuid,timestamptz,text)') IS NOT NULL THEN
  ALTER FUNCTION controlled_lock_owned_account_v1(uuid,uuid,text) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_update_account_v1(uuid,text,uuid,bigint,text,text,text,text,uuid,timestamptz,text) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_lock_funding_policy_v1(uuid,uuid,text) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_lock_correction_original_v1(uuid,uuid) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_lock_owned_account_v1(uuid,uuid,text),controlled_update_account_v1(uuid,text,uuid,bigint,text,text,text,text,uuid,timestamptz,text),controlled_lock_funding_policy_v1(uuid,uuid,text),controlled_lock_correction_original_v1(uuid,uuid) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_lock_owned_account_v1(uuid,uuid,text),controlled_update_account_v1(uuid,text,uuid,bigint,text,text,text,text,uuid,timestamptz,text),controlled_lock_funding_policy_v1(uuid,uuid,text),controlled_lock_correction_original_v1(uuid,uuid) TO ledgersync_api;
  REVOKE UPDATE ON accounts,account_balance_projections,transfers FROM ledgersync_api;
  REVOKE INSERT ON transfers,journal_transactions,ledger_postings,transfer_velocity_events,transfer_velocity_totals FROM ledgersync_api;
  REVOKE UPDATE,DELETE ON transfer_velocity_events,transfer_velocity_totals FROM ledgersync_api;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_apply_partner_v1(uuid,text,uuid,text,text,timestamptz)') IS NOT NULL THEN
  GRANT SELECT,INSERT ON partner_provisioning_requests,partner_credential_events TO ledgersync_migration_owner;
  GRANT UPDATE ON partner_provisioning_requests TO ledgersync_migration_owner;
  GRANT INSERT ON tenants,tenant_transfer_policies,transfer_policy_versions,tenant_subject_roles TO ledgersync_migration_owner;
  GRANT SELECT ON transfer_policy_versions TO ledgersync_migration_owner;
  GRANT DELETE ON account_owners,account_credit_permissions,tenant_subject_roles TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_apply_partner_v1(uuid,text,uuid,text,text,timestamptz) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_rollback_partner_v1(uuid,text,uuid,timestamptz) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_apply_partner_v1(uuid,text,uuid,text,text,timestamptz),controlled_rollback_partner_v1(uuid,text,uuid,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_apply_partner_v1(uuid,text,uuid,text,text,timestamptz),controlled_rollback_partner_v1(uuid,text,uuid,timestamptz) TO ledgersync_provisioning;
  REVOKE INSERT,UPDATE,DELETE ON accounts,account_opening_balances,account_balance_projections,account_owners,account_credit_permissions,tenants,tenant_transfer_policies,tenant_subject_roles,partner_credential_events,partner_provisioning_requests,audit_events FROM ledgersync_provisioning;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz)') IS NOT NULL THEN
  ALTER FUNCTION controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) TO ledgersync_api,ledgersync_worker,ledgersync_reconciliation,ledgersync_scheduler;
  REVOKE INSERT ON audit_events FROM ledgersync_api,ledgersync_worker,ledgersync_reconciliation,ledgersync_scheduler;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_request_funding_v1(uuid,text,uuid,text,bigint,text,text,text,bytea,uuid,timestamptz)') IS NOT NULL THEN
  GRANT INSERT ON funding_events TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_fingerprint_parts_v1(text[]) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_request_funding_v1(uuid,text,uuid,text,bigint,text,text,text,bytea,uuid,timestamptz) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_request_funding_compensation_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_decide_funding_v1(uuid,text,uuid,text,boolean,text,uuid,timestamptz) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_fingerprint_parts_v1(text[]),controlled_request_funding_v1(uuid,text,uuid,text,bigint,text,text,text,bytea,uuid,timestamptz),controlled_request_funding_compensation_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz),controlled_decide_funding_v1(uuid,text,uuid,text,boolean,text,uuid,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_request_funding_v1(uuid,text,uuid,text,bigint,text,text,text,bytea,uuid,timestamptz),controlled_request_funding_compensation_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz),controlled_decide_funding_v1(uuid,text,uuid,text,boolean,text,uuid,timestamptz) TO ledgersync_api;
  REVOKE INSERT,UPDATE,DELETE ON funding_events,funding_velocity_events FROM ledgersync_api;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_request_correction_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz,timestamptz)') IS NOT NULL THEN
  GRANT INSERT ON transfer_corrections TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_request_correction_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz,timestamptz) OWNER TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_decide_correction_v1(uuid,text,uuid,text,text,uuid,timestamptz,timestamptz) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_request_correction_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz,timestamptz),controlled_decide_correction_v1(uuid,text,uuid,text,text,uuid,timestamptz,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_request_correction_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz,timestamptz),controlled_decide_correction_v1(uuid,text,uuid,text,text,uuid,timestamptz,timestamptz) TO ledgersync_api;
  REVOKE INSERT,UPDATE,DELETE ON transfer_corrections,approval_records FROM ledgersync_api;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_renew_claim_v1(text,uuid,text,bigint,double precision)') IS NOT NULL THEN
  GRANT UPDATE ON outbox_publication_attempts TO ledgersync_queue_owner;
  GRANT INSERT ON delivery_attempts,logical_message_receipts TO ledgersync_queue_owner;
  GRANT EXECUTE ON FUNCTION advance_logical_clock(text,bigint) TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_renew_claim_v1(text,uuid,text,bigint,double precision) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_complete_outbox_v1(uuid,text,bigint,timestamptz,text,text) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_complete_delivery_v1(uuid,text,bigint,integer,text,timestamptz,timestamptz,text,text) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_prepare_publication_v1(uuid,text,bigint,text) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_receive_message_v1(uuid,text,uuid,text,text,bigint) OWNER TO ledgersync_queue_owner;
  REVOKE ALL ON FUNCTION controlled_renew_claim_v1(text,uuid,text,bigint,double precision),controlled_complete_outbox_v1(uuid,text,bigint,timestamptz,text,text),controlled_complete_delivery_v1(uuid,text,bigint,integer,text,timestamptz,timestamptz,text,text),controlled_prepare_publication_v1(uuid,text,bigint,text),controlled_receive_message_v1(uuid,text,uuid,text,text,bigint) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_renew_claim_v1(text,uuid,text,bigint,double precision),controlled_complete_outbox_v1(uuid,text,bigint,timestamptz,text,text),controlled_complete_delivery_v1(uuid,text,bigint,integer,text,timestamptz,timestamptz,text,text),controlled_prepare_publication_v1(uuid,text,bigint,text),controlled_receive_message_v1(uuid,text,uuid,text,text,bigint) TO ledgersync_worker;
  REVOKE UPDATE ON outbox_events,outbox_publication_attempts,webhook_delivery_jobs,webhook_endpoint_verification_jobs FROM ledgersync_worker;
  REVOKE INSERT ON delivery_attempts,outbox_publication_attempts,logical_message_receipts FROM ledgersync_worker;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_outbox_recovery_v1(uuid,text,uuid,text,text,uuid,timestamptz)') IS NOT NULL THEN
  GRANT SELECT ON tenant_subject_roles TO ledgersync_queue_owner;
  GRANT SELECT,INSERT,UPDATE ON outbox_replay_actions,delivery_replay_actions TO ledgersync_queue_owner;
  GRANT SELECT,UPDATE ON delivery_attempts TO ledgersync_queue_owner;
  GRANT INSERT ON webhook_delivery_jobs TO ledgersync_queue_owner;
  GRANT EXECUTE ON FUNCTION controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_outbox_recovery_v1(uuid,text,uuid,text,text,uuid,timestamptz) OWNER TO ledgersync_queue_owner;
  ALTER FUNCTION controlled_delivery_recovery_v1(uuid,text,uuid,text,uuid,text,text,uuid,timestamptz) OWNER TO ledgersync_queue_owner;
  REVOKE ALL ON FUNCTION controlled_outbox_recovery_v1(uuid,text,uuid,text,text,uuid,timestamptz),controlled_delivery_recovery_v1(uuid,text,uuid,text,uuid,text,text,uuid,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_outbox_recovery_v1(uuid,text,uuid,text,text,uuid,timestamptz),controlled_delivery_recovery_v1(uuid,text,uuid,text,uuid,text,text,uuid,timestamptz) TO ledgersync_api;
  REVOKE INSERT,UPDATE,DELETE ON outbox_replay_actions,delivery_replay_actions,webhook_delivery_jobs FROM ledgersync_api;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_record_reconciliation_v1(uuid,uuid,text,bigint,bigint,uuid,timestamptz,timestamptz,jsonb,text,text,text,jsonb)') IS NOT NULL THEN
  GRANT SELECT ON outbox_events TO ledgersync_migration_owner;
  GRANT INSERT ON reconciliation_runs,reconciliation_mismatches TO ledgersync_migration_owner;
  GRANT EXECUTE ON FUNCTION scheduler_tenant_allowed(uuid) TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_record_reconciliation_v1(uuid,uuid,text,bigint,bigint,uuid,timestamptz,timestamptz,jsonb,text,text,text,jsonb) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_record_reconciliation_v1(uuid,uuid,text,bigint,bigint,uuid,timestamptz,timestamptz,jsonb,text,text,text,jsonb) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_record_reconciliation_v1(uuid,uuid,text,bigint,bigint,uuid,timestamptz,timestamptz,jsonb,text,text,text,jsonb) TO ledgersync_api,ledgersync_reconciliation,ledgersync_scheduler;
  REVOKE INSERT ON reconciliation_runs,reconciliation_mismatches FROM ledgersync_api,ledgersync_reconciliation,ledgersync_scheduler;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_run_retention_v1(uuid,boolean,integer,double precision,double precision,uuid,text,timestamptz)') IS NOT NULL THEN
  GRANT USAGE ON SCHEMA public TO ledgersync_maintenance,ledgersync_maintenance_owner;
  GRANT SELECT ON outbox_events,delivery_attempts,webhook_delivery_jobs,outbox_replay_actions,outbox_publication_attempts,logical_message_receipts,idempotency_requests,api_rate_limit_windows TO ledgersync_maintenance_owner;
  GRANT UPDATE,DELETE ON outbox_events,api_rate_limit_windows TO ledgersync_maintenance_owner;
  GRANT INSERT ON retention_runs TO ledgersync_maintenance_owner;
  GRANT EXECUTE ON FUNCTION controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) TO ledgersync_maintenance_owner;
  ALTER FUNCTION controlled_run_retention_v1(uuid,boolean,integer,double precision,double precision,uuid,text,timestamptz) OWNER TO ledgersync_maintenance_owner;
  REVOKE ALL ON FUNCTION controlled_run_retention_v1(uuid,boolean,integer,double precision,double precision,uuid,text,timestamptz) FROM PUBLIC;
  GRANT EXECUTE ON FUNCTION controlled_run_retention_v1(uuid,boolean,integer,double precision,double precision,uuid,text,timestamptz) TO ledgersync_maintenance;
  REVOKE INSERT ON retention_runs FROM ledgersync_api;
 END IF;
END $$;

DO $$ BEGIN
 IF to_regprocedure('public.controlled_import_delivery_evidence_v1(uuid,uuid,uuid,uuid,text,text,integer,text,text,text,timestamptz,timestamptz,timestamptz)') IS NOT NULL THEN
  GRANT INSERT ON delivery_attempts TO ledgersync_migration_owner;
  ALTER FUNCTION controlled_import_delivery_evidence_v1(uuid,uuid,uuid,uuid,text,text,integer,text,text,text,timestamptz,timestamptz,timestamptz) OWNER TO ledgersync_migration_owner;
  REVOKE ALL ON FUNCTION controlled_import_delivery_evidence_v1(uuid,uuid,uuid,uuid,text,text,integer,text,text,text,timestamptz,timestamptz,timestamptz) FROM PUBLIC;
 END IF;
END $$;
