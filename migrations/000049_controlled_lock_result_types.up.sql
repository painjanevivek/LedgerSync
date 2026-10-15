-- Forward repair: table currency is CHAR(3); capability results use text.
CREATE OR REPLACE FUNCTION controlled_lock_owned_account_v1(p_tenant uuid,p_account uuid,p_actor text)
RETURNS TABLE(id uuid,tenant_id uuid,currency text,status text,display_name text,external_reference text,category text,
 version bigint,created_at timestamptz,updated_at timestamptz,closed_at timestamptz,available_minor bigint,ledger_minor bigint,permission text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles r WHERE r.tenant_id=p_tenant AND r.subject_id=p_actor AND r.role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'account actor denied' USING ERRCODE='42501';
 END IF;
 RETURN QUERY SELECT a.id,a.tenant_id,a.currency::text,a.status::text,a.display_name::text,a.external_reference::text,a.category::text,a.version,a.created_at,a.updated_at,a.closed_at,
 b.available_minor,b.ledger_minor,o.permission::text
 FROM public.accounts a JOIN public.account_balance_projections b ON b.account_id=a.id
 JOIN public.account_owners o ON o.tenant_id=a.tenant_id AND o.account_id=a.id AND o.subject_id=p_actor AND o.permission='debit'
 WHERE a.tenant_id=p_tenant AND a.id=p_account AND a.account_kind='customer' FOR UPDATE OF a,b;
END $$;
CREATE OR REPLACE FUNCTION controlled_lock_funding_policy_v1(p_tenant uuid,p_account uuid,p_currency text)
RETURNS TABLE(mode text,finance_activated boolean,policy_version bigint,per_command_minor bigint,operator_rolling_24h_minor bigint,tenant_rolling_24h_minor bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles
  WHERE tenant_id=p_tenant AND subject_id=current_setting('app.actor_id',true) AND role='finance') THEN
  RAISE EXCEPTION 'funding actor denied' USING ERRCODE='42501';
 END IF;
 RETURN QUERY SELECT p.mode::text,p.finance_activated,p.policy_version::bigint,p.per_command_minor,p.operator_rolling_24h_minor,p.tenant_rolling_24h_minor
 FROM public.accounts a JOIN public.tenant_funding_policies p ON p.tenant_id=a.tenant_id AND p.currency=a.currency
 WHERE a.tenant_id=p_tenant AND a.id=p_account AND a.currency=p_currency AND a.status='active' AND a.account_kind='customer' FOR UPDATE OF a;
END $$;
CREATE OR REPLACE FUNCTION controlled_lock_correction_original_v1(p_tenant uuid,p_transfer uuid)
RETURNS TABLE(status text,compensation_of text,policy_version bigint,control_mode text,requires_step_up boolean,approval_ttl_minutes integer)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles
  WHERE tenant_id=p_tenant AND subject_id=current_setting('app.actor_id',true) AND role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'correction actor denied' USING ERRCODE='42501';
 END IF;
 RETURN QUERY SELECT t.status::text,COALESCE(t.compensation_of_transfer_id::text,''),p.policy_version::bigint,p.control_mode::text,p.requires_step_up,p.approval_ttl_minutes::integer
 FROM public.transfers t JOIN public.tenant_transfer_policies p ON p.tenant_id=t.tenant_id
 WHERE t.tenant_id=p_tenant AND t.id=p_transfer FOR UPDATE OF t;
END $$;
