-- Tenant settings are defence against query omissions, not credentials.
-- Only the authenticated application boundary may choose tenant and actor.
CREATE FUNCTION tenant_scope_allows(p_tenant uuid) RETURNS boolean
LANGUAGE sql STABLE SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT p_tenant IS NOT NULL AND p_tenant<>'00000000-0000-0000-0000-000000000000'::uuid
 AND p_tenant::text=current_setting('app.tenant_id',true)
 AND length(current_setting('app.actor_id',true)) BETWEEN 1 AND 256
 AND current_setting('app.actor_id',true)=btrim(current_setting('app.actor_id',true))
 AND current_setting('app.actor_id',true) !~ '[[:cntrl:]]'
$$;

DO $body$ DECLARE relation text; BEGIN
 FOREACH relation IN ARRAY ARRAY['account_credit_permissions','account_owners','accounts','api_rate_limit_windows','approval_records','audit_events','delivery_attempts','delivery_replay_actions','developer_command_idempotency','developer_credential_events','developer_credentials','developer_webhook_command_idempotency','developer_webhook_endpoints','developer_webhook_events','funding_events','funding_velocity_events','idempotency_requests','investigation_live_room_findings','investigation_live_room_operations','investigation_live_rooms','investigation_saved_views','investigation_workspace_references','investigation_workspaces','journal_transactions','ledger_postings','logical_message_receipts','operator_onboarding_preferences','outbox_events','outbox_replay_actions','partner_credential_events','partner_provisioning_requests','reconciliation_mismatches','reconciliation_run_commands','reconciliation_runs','reconciliation_scheduled_jobs','reconciliation_scheduler_leases','reconciliation_scheduler_tenants','retention_runs','tenant_funding_policies','tenant_subject_roles','tenant_transfer_policies','transfer_corrections','transfer_policy_versions','transfer_recovery_intents','transfer_velocity_events','transfer_velocity_totals','transfers','webhook_delivery_jobs','webhook_endpoint_verification_jobs'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',relation);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',relation);
  EXECUTE format('CREATE POLICY tenant_scope ON public.%I USING(public.tenant_scope_allows(tenant_id)) WITH CHECK(public.tenant_scope_allows(tenant_id))',relation);
 END LOOP;
END $body$;
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON tenants USING(public.tenant_scope_allows(id)) WITH CHECK(public.tenant_scope_allows(id));

-- The boolean helper avoids granting provisioning users a new account read API.
-- Its function owner is also subject to the account tenant policy.
CREATE FUNCTION tenant_scope_account(p_account uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT EXISTS(SELECT 1 FROM public.accounts WHERE id=p_account AND public.tenant_scope_allows(tenant_id))
$$;
ALTER TABLE account_balance_projections ENABLE ROW LEVEL SECURITY;
ALTER TABLE account_balance_projections FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON account_balance_projections USING(public.tenant_scope_account(account_id)) WITH CHECK(public.tenant_scope_account(account_id));
ALTER TABLE account_opening_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE account_opening_balances FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON account_opening_balances USING(public.tenant_scope_account(account_id)) WITH CHECK(public.tenant_scope_account(account_id));
ALTER TABLE outbox_publication_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_publication_attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON outbox_publication_attempts USING(EXISTS(SELECT 1 FROM public.outbox_events e WHERE e.id=event_id AND public.tenant_scope_allows(e.tenant_id))) WITH CHECK(EXISTS(SELECT 1 FROM public.outbox_events e WHERE e.id=event_id AND public.tenant_scope_allows(e.tenant_id)));

-- Only bounded queue discovery functions use this role. It has no financial ACL.
CREATE POLICY queue_discovery ON outbox_events TO ledgersync_queue_owner USING(true) WITH CHECK(true);
CREATE POLICY queue_discovery ON webhook_delivery_jobs TO ledgersync_queue_owner USING(true) WITH CHECK(true);
CREATE POLICY queue_discovery ON webhook_endpoint_verification_jobs TO ledgersync_queue_owner USING(true) WITH CHECK(true);
CREATE POLICY queue_discovery ON outbox_publication_attempts TO ledgersync_queue_owner USING(true) WITH CHECK(true);
CREATE POLICY queue_endpoint_read ON developer_webhook_endpoints FOR SELECT TO ledgersync_queue_owner USING(true);

-- Startup needs a global currency invariant, without exposing tenant records.
DO $$ BEGIN CREATE ROLE ledgersync_preflight_owner NOLOGIN NOSUPERUSER NOBYPASSRLS; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE POLICY preflight_read ON tenants FOR SELECT TO ledgersync_preflight_owner USING(true);
CREATE POLICY preflight_read ON tenant_transfer_policies FOR SELECT TO ledgersync_preflight_owner USING(true);
CREATE POLICY preflight_read ON accounts FOR SELECT TO ledgersync_preflight_owner USING(true);
CREATE POLICY preflight_read ON transfers FOR SELECT TO ledgersync_preflight_owner USING(true);
CREATE FUNCTION controlled_pilot_currency_valid_v1(p_currency text) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT p_currency ~ '^[A-Z]{3}$'
 AND NOT EXISTS(SELECT 1 FROM public.tenants t LEFT JOIN public.tenant_transfer_policies p ON p.tenant_id=t.id WHERE p.tenant_id IS NULL)
 AND NOT EXISTS(SELECT 1 FROM public.tenant_transfer_policies WHERE currency<>p_currency)
 AND NOT EXISTS(SELECT 1 FROM public.accounts WHERE currency<>p_currency)
 AND NOT EXISTS(SELECT 1 FROM public.transfers WHERE currency<>p_currency)
$$;
REVOKE ALL ON FUNCTION controlled_pilot_currency_valid_v1(text) FROM PUBLIC;
