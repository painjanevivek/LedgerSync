-- Narrow locking and lifecycle capabilities replace ordinary financial UPDATE.
CREATE FUNCTION controlled_lock_owned_account_v1(p_tenant uuid,p_account uuid,p_actor text)
RETURNS TABLE(id uuid,tenant_id uuid,currency text,status text,display_name text,external_reference text,category text,
 version bigint,created_at timestamptz,updated_at timestamptz,closed_at timestamptz,available_minor bigint,ledger_minor bigint,permission text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles r WHERE r.tenant_id=p_tenant AND r.subject_id=p_actor AND r.role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'account actor denied' USING ERRCODE='42501';
 END IF;
 RETURN QUERY SELECT a.id,a.tenant_id,a.currency,a.status,a.display_name,a.external_reference,a.category,a.version,a.created_at,a.updated_at,a.closed_at,
 b.available_minor,b.ledger_minor,o.permission
 FROM public.accounts a JOIN public.account_balance_projections b ON b.account_id=a.id
 JOIN public.account_owners o ON o.tenant_id=a.tenant_id AND o.account_id=a.id AND o.subject_id=p_actor AND o.permission='debit'
 WHERE a.tenant_id=p_tenant AND a.id=p_account AND a.account_kind='customer' FOR UPDATE OF a,b;
END $$;

CREATE FUNCTION controlled_update_account_v1(p_tenant uuid,p_actor text,p_account uuid,p_version bigint,
 p_name text,p_reference text,p_category text,p_status text,p_correlation uuid,p_at timestamptz,p_reason text)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE a record; authoritative numeric; audit_type text; event_type text; metadata jsonb; event_id uuid:=gen_random_uuid();
BEGIN
 SELECT * INTO a FROM public.controlled_lock_owned_account_v1(p_tenant,p_account,p_actor);
 IF NOT FOUND THEN RAISE EXCEPTION 'account not found' USING ERRCODE='42501'; END IF;
 IF p_version IS NULL OR p_version<>a.version THEN RETURN false; END IF;
 IF a.status='closed' OR p_status IS NULL OR p_status NOT IN ('active','frozen','closed')
  OR p_at IS NULL OR p_at<a.updated_at OR p_correlation IS NULL
  OR p_name IS NULL OR char_length(p_name) NOT BETWEEN 1 AND 120 OR btrim(p_name)<>p_name OR p_name~'[[:cntrl:]]'
  OR p_reference IS NULL OR p_reference!~'^[a-z0-9][a-z0-9._-]{2,63}$'
  OR p_category IS NULL OR p_category NOT IN ('operating','customer_funds','payroll','payables','expenses','reserve') THEN
  RAISE EXCEPTION 'invalid account mutation' USING ERRCODE='22023';
 END IF;
 IF p_status<>a.status THEN
  IF (p_name,p_reference,p_category) IS DISTINCT FROM (a.display_name,a.external_reference,a.category)
   OR p_reason IS NULL OR char_length(p_reason) NOT BETWEEN 1 AND 256 OR p_reason~'[[:cntrl:]]' THEN
   RAISE EXCEPTION 'invalid lifecycle intent' USING ERRCODE='22023';
  END IF;
  IF p_status='closed' THEN
   IF EXISTS(SELECT 1 FROM public.transfers WHERE tenant_id=p_tenant AND status='pending' AND (debit_account_id=p_account OR credit_account_id=p_account))
    OR EXISTS(SELECT 1 FROM public.funding_events WHERE tenant_id=p_tenant AND status IN ('requested','approved') AND destination_account_id=p_account)
    OR EXISTS(SELECT 1 FROM public.transfer_corrections c JOIN public.transfers t ON t.id=c.original_transfer_id AND t.tenant_id=c.tenant_id
      WHERE c.tenant_id=p_tenant AND c.status IN ('requested','approved') AND (t.debit_account_id=p_account OR t.credit_account_id=p_account)) THEN
    RAISE EXCEPTION 'account has unresolved obligations' USING ERRCODE='23514';
   END IF;
   SELECT o.opening_ledger_minor::numeric+COALESCE(sum(CASE WHEN p.direction='credit' THEN p.amount_minor::numeric ELSE -p.amount_minor::numeric END),0)
    INTO authoritative FROM public.account_opening_balances o LEFT JOIN public.ledger_postings p ON p.account_id=o.account_id
    WHERE o.account_id=p_account GROUP BY o.opening_ledger_minor;
   IF authoritative IS NULL OR authoritative<>0 OR a.ledger_minor<>0 OR a.available_minor<>0 THEN
    RAISE EXCEPTION 'account closure requires a consistent zero balance' USING ERRCODE='23514';
   END IF;
  END IF;
  audit_type:='account.status_changed'; event_type:='account.status.changed.v1';
  metadata:=jsonb_build_object('status',p_status,'reason',p_reason);
 ELSE
  IF p_reason IS DISTINCT FROM '' THEN RAISE EXCEPTION 'metadata intent must not carry a lifecycle reason' USING ERRCODE='22023'; END IF;
  audit_type:='account.metadata_updated'; event_type:='account.metadata.updated.v1';
  metadata:=jsonb_build_object('status',p_status,'category',p_category);
 END IF;
 UPDATE public.accounts SET display_name=p_name,external_reference=p_reference,category=p_category,status=p_status,
  closed_at=CASE WHEN p_status='closed' THEN p_at ELSE NULL END,version=a.version+1,updated_at=p_at WHERE id=p_account AND tenant_id=p_tenant;
 INSERT INTO public.audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)
 VALUES(gen_random_uuid(),p_tenant,p_actor,audit_type,'account',p_account::text,'succeeded',p_correlation,metadata,p_at);
 INSERT INTO public.outbox_events(id,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at)
 VALUES(event_id,p_tenant,NULL,p_account,'account',p_account,event_type,a.version+1,
  jsonb_build_object('event_id',event_id::text,'event_type',event_type,'aggregate_type','account','aggregate_id',p_account::text,
   'account_id',p_account::text,'currency',a.currency,'status',p_status,'version',(a.version+1)::text,
   'occurred_at',to_char(p_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')) ||
   CASE WHEN p_status<>a.status THEN jsonb_build_object('reason',p_reason) ELSE '{}'::jsonb END,p_at);
 RETURN true;
END $$;

CREATE FUNCTION controlled_lock_funding_policy_v1(p_tenant uuid,p_account uuid,p_currency text)
RETURNS TABLE(mode text,finance_activated boolean,policy_version bigint,per_command_minor bigint,operator_rolling_24h_minor bigint,tenant_rolling_24h_minor bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles
  WHERE tenant_id=p_tenant AND subject_id=current_setting('app.actor_id',true) AND role='finance') THEN
  RAISE EXCEPTION 'funding actor denied' USING ERRCODE='42501';
 END IF;
 RETURN QUERY SELECT p.mode,p.finance_activated,p.policy_version,p.per_command_minor,p.operator_rolling_24h_minor,p.tenant_rolling_24h_minor
 FROM public.accounts a JOIN public.tenant_funding_policies p ON p.tenant_id=a.tenant_id AND p.currency=a.currency
 WHERE a.tenant_id=p_tenant AND a.id=p_account AND a.currency=p_currency AND a.status='active' AND a.account_kind='customer' FOR UPDATE OF a;
END $$;

CREATE FUNCTION controlled_lock_correction_original_v1(p_tenant uuid,p_transfer uuid)
RETURNS TABLE(status text,compensation_of text,policy_version bigint,control_mode text,requires_step_up boolean,approval_ttl_minutes integer)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles
  WHERE tenant_id=p_tenant AND subject_id=current_setting('app.actor_id',true) AND role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'correction actor denied' USING ERRCODE='42501';
 END IF;
 RETURN QUERY SELECT t.status,COALESCE(t.compensation_of_transfer_id::text,''),p.policy_version,p.control_mode,p.requires_step_up,p.approval_ttl_minutes
 FROM public.transfers t JOIN public.tenant_transfer_policies p ON p.tenant_id=t.tenant_id
 WHERE t.tenant_id=p_tenant AND t.id=p_transfer FOR UPDATE OF t;
END $$;
REVOKE ALL ON FUNCTION controlled_lock_owned_account_v1(uuid,uuid,text),controlled_update_account_v1(uuid,text,uuid,bigint,text,text,text,text,uuid,timestamptz,text),
 controlled_lock_funding_policy_v1(uuid,uuid,text),controlled_lock_correction_original_v1(uuid,uuid) FROM PUBLIC;
