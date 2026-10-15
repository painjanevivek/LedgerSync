-- No caller can choose an opening value or turn a customer account into a
-- clearing account. Creation participates in the caller's funding transaction.
CREATE FUNCTION controlled_ensure_funding_account_v1(
 p_tenant uuid,p_actor text,p_currency text,p_correlation uuid,p_at timestamptz
) RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE account_id uuid; created_id uuid;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true
  OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR p_correlation IS NULL OR p_at IS NULL THEN
  RAISE EXCEPTION 'funding account scope denied' USING ERRCODE='42501',CONSTRAINT='controlled_funding_actor';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role='finance')
  OR NOT EXISTS(SELECT 1 FROM public.tenant_funding_policies WHERE tenant_id=p_tenant AND currency=p_currency AND (mode='local_demo_single_operator' OR finance_activated)) THEN
  RAISE EXCEPTION 'funding account actor or policy denied' USING ERRCODE='42501',CONSTRAINT='controlled_funding_actor';
 END IF;
 SELECT id INTO account_id FROM public.accounts WHERE tenant_id=p_tenant AND currency=p_currency AND account_kind='funding_clearing' FOR UPDATE;
 IF FOUND THEN RETURN account_id; END IF;
 created_id:=gen_random_uuid();
 INSERT INTO public.accounts(id,tenant_id,currency,status,display_name,category,external_reference,account_kind,created_at,updated_at)
 VALUES(created_id,p_tenant,p_currency,'active','Funding clearing · '||p_currency,'system','system-funding-'||lower(p_currency),'funding_clearing',p_at,p_at)
 ON CONFLICT(tenant_id,currency) WHERE account_kind='funding_clearing' DO NOTHING RETURNING id INTO account_id;
 -- A serializable caller can lose creation to a different actor's request.
 -- Restart the whole command; do not reuse an older snapshot or partial state.
 IF NOT FOUND THEN RAISE EXCEPTION 'funding account creation requires a new snapshot' USING ERRCODE='40001'; END IF;
 INSERT INTO public.account_balance_projections(account_id,available_minor,ledger_minor,balance_version,allow_negative,updated_at) VALUES(created_id,0,0,0,true,p_at);
 INSERT INTO public.account_opening_balances(account_id,opening_ledger_minor) VALUES(created_id,0);
 INSERT INTO public.audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)
 VALUES(gen_random_uuid(),p_tenant,p_actor,'account.funding_clearing_created','account',created_id::text,'succeeded',p_correlation,
  jsonb_build_object('function','controlled_ensure_funding_account_v1','account_kind','funding_clearing'),p_at);
 RETURN created_id;
END $$;
REVOKE ALL ON FUNCTION controlled_ensure_funding_account_v1(uuid,text,text,uuid,timestamptz) FROM PUBLIC;
