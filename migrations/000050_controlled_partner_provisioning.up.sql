-- Provisioning is a local administrative command. No workload receives the
-- function owner's role or direct financial table write permission.
CREATE FUNCTION controlled_apply_partner_v1(p_tenant uuid,p_actor text,p_correlation uuid,p_config text,p_currency text,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE c jsonb; subject jsonb; credential jsonb; account jsonb; fingerprint bytea;
 prior bytea; request_id uuid:=gen_random_uuid(); details jsonb; outcome record;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER') OR p_correlation IS NULL OR p_at IS NULL THEN
  RAISE EXCEPTION 'provisioning actor denied' USING ERRCODE='42501';
 END IF;
 IF p_config IS NULL OR octet_length(p_config)>4194304 THEN RAISE EXCEPTION 'invalid provisioning configuration size' USING ERRCODE='22023'; END IF;
 c:=p_config::jsonb;
 IF jsonb_typeof(c) IS DISTINCT FROM 'object' OR (c->>'tenant_id')::uuid IS DISTINCT FROM p_tenant
  OR c->>'currency' IS DISTINCT FROM p_currency OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
  OR jsonb_typeof(c->'accounts') IS DISTINCT FROM 'array' OR jsonb_array_length(c->'accounts') NOT BETWEEN 1 AND 10000
  OR jsonb_typeof(c->'subjects') IS DISTINCT FROM 'array' OR jsonb_array_length(c->'subjects') NOT BETWEEN 1 AND 10000
  OR jsonb_typeof(c->'credentials') IS DISTINCT FROM 'array' OR jsonb_array_length(c->'credentials')>10000 THEN
  RAISE EXCEPTION 'invalid provisioning configuration' USING ERRCODE='22023';
 END IF;
 fingerprint:=public.digest(convert_to(p_config,'UTF8'),'sha256');
 SELECT configuration_fingerprint INTO prior FROM public.partner_provisioning_requests WHERE tenant_id=p_tenant AND correlation_id=p_correlation AND status='applied';
 IF FOUND THEN
  IF prior=fingerprint THEN RETURN; END IF;
  RAISE EXCEPTION 'provisioning correlation belongs to different configuration' USING ERRCODE='23505';
 END IF;
 INSERT INTO public.tenants(id,external_reference) VALUES(p_tenant,c->>'external_reference');
 INSERT INTO public.tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor)
 VALUES(p_tenant,p_currency,(c->>'minimum_transfer_minor')::bigint,(c->>'maximum_transfer_minor')::bigint,
 (c->>'actor_rolling_24h_minor')::bigint,(c->>'source_rolling_24h_minor')::bigint,(c->>'tenant_rolling_24h_minor')::bigint);
 FOR subject IN SELECT value FROM jsonb_array_elements(c->'subjects') LOOP
  IF subject->>'id' IS NULL OR btrim(subject->>'id')='' OR jsonb_typeof(subject->'roles') IS DISTINCT FROM 'array'
   OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(subject->'roles') r WHERE r NOT IN ('operator','finance','support','viewer')) THEN
   RAISE EXCEPTION 'invalid provisioning subject' USING ERRCODE='22023';
  END IF;
  INSERT INTO public.tenant_subject_roles(tenant_id,subject_id,role) SELECT p_tenant,subject->>'id',value FROM jsonb_array_elements_text(subject->'roles');
 END LOOP;
 FOR credential IN SELECT value FROM jsonb_array_elements(c->'credentials') LOOP
  IF COALESCE(credential->>'reference','')='' OR COALESCE(credential->>'audience','')=''
   OR (credential->>'expires_at')::timestamptz IS NULL OR (credential->>'expires_at')::timestamptz<=clock_timestamp()
   OR jsonb_typeof(credential->'scopes') IS DISTINCT FROM 'array' OR jsonb_array_length(credential->'scopes')=0
   OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(credential->'scopes') s WHERE s NOT IN ('accounts:read','transactions:read','reconciliation:read','transfers:read','transfers:write')) THEN
   RAISE EXCEPTION 'invalid provisioning credential reference' USING ERRCODE='22023';
  END IF;
  INSERT INTO public.partner_credential_events(id,tenant_id,credential_reference,action,audience,scopes,expires_at,actor_subject_id,correlation_id)
  VALUES(gen_random_uuid(),p_tenant,credential->>'reference','registered',credential->>'audience',ARRAY(SELECT jsonb_array_elements_text(credential->'scopes')),(credential->>'expires_at')::timestamptz,p_actor,p_correlation);
 END LOOP;
 FOR account IN SELECT value FROM jsonb_array_elements(c->'accounts') LOOP
  IF account->>'opening_minor' IS DISTINCT FROM '0' THEN RAISE EXCEPTION 'normal provisioning requires exact zero opening' USING ERRCODE='22023'; END IF;
  SELECT * INTO outcome FROM public.controlled_provision_account_v1(p_tenant,p_actor,(account->>'id')::uuid,p_currency,
   account->>'display_name',account->>'category',COALESCE(NULLIF(btrim(account->>'external_reference'),''),'provisioned-'||lower(account->>'id')),
   ARRAY(SELECT jsonb_array_elements_text(NULLIF(account->'read_subjects','null'::jsonb))),
   ARRAY(SELECT jsonb_array_elements_text(NULLIF(account->'debit_subjects','null'::jsonb))),
   ARRAY(SELECT jsonb_array_elements_text(NULLIF(account->'credit_subjects','null'::jsonb))),p_correlation,p_at);
  IF outcome.replayed OR outcome.conflicted THEN RAISE EXCEPTION 'account conflict during new provisioning' USING ERRCODE='23505'; END IF;
 END LOOP;
 details:=jsonb_build_object('external_reference',c->>'external_reference','subject_count',jsonb_array_length(c->'subjects'));
 INSERT INTO public.partner_provisioning_requests(id,tenant_id,actor_subject_id,correlation_id,configuration_fingerprint,status,currency,account_count,sanitized_details,created_at)
 VALUES(request_id,p_tenant,p_actor,p_correlation,fingerprint,'applied',p_currency,jsonb_array_length(c->'accounts'),details,p_at);
 INSERT INTO public.audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)
 VALUES(request_id,p_tenant,p_actor,'partner.provisioned','tenant',p_tenant::text,'succeeded',p_correlation,details,p_at);
END $$;

CREATE FUNCTION controlled_rollback_partner_v1(p_tenant uuid,p_actor text,p_correlation uuid,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE request public.partner_provisioning_requests%ROWTYPE; credential record; request_id uuid:=gen_random_uuid();
 details jsonb:='{"rollback":"no_financial_activity"}';
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER') OR p_correlation IS NULL OR p_at IS NULL THEN
  RAISE EXCEPTION 'provisioning actor denied' USING ERRCODE='42501';
 END IF;
 IF EXISTS(SELECT 1 FROM public.partner_provisioning_requests WHERE tenant_id=p_tenant AND correlation_id=p_correlation AND status='rolled_back') THEN RETURN; END IF;
 SELECT * INTO request FROM public.partner_provisioning_requests WHERE tenant_id=p_tenant AND correlation_id=p_correlation AND status='applied' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'applied provisioning request not found' USING ERRCODE='P0002'; END IF;
 -- Match posting lock order. SERIALIZABLE callers retry a racing financial command.
 PERFORM id FROM public.accounts WHERE tenant_id=p_tenant ORDER BY id FOR UPDATE;
 PERFORM b.account_id FROM public.account_balance_projections b JOIN public.accounts a ON a.id=b.account_id WHERE a.tenant_id=p_tenant ORDER BY b.account_id FOR UPDATE OF b;
 IF EXISTS(SELECT 1 FROM public.transfers WHERE tenant_id=p_tenant)
  OR EXISTS(SELECT 1 FROM public.funding_events WHERE tenant_id=p_tenant)
  OR EXISTS(SELECT 1 FROM public.journal_transactions WHERE tenant_id=p_tenant)
  OR EXISTS(SELECT 1 FROM public.ledger_postings WHERE tenant_id=p_tenant)
  OR EXISTS(SELECT 1 FROM public.accounts a JOIN public.account_balance_projections b ON b.account_id=a.id JOIN public.account_opening_balances o ON o.account_id=a.id
    WHERE a.tenant_id=p_tenant AND (b.available_minor<>0 OR b.ledger_minor<>0 OR o.opening_ledger_minor<>0))
  OR (SELECT count(*) FROM public.accounts WHERE tenant_id=p_tenant)<>request.account_count THEN
  RAISE EXCEPTION 'tenant with financial activity or changed account set cannot be rolled back' USING ERRCODE='23514';
 END IF;
 FOR credential IN SELECT e.* FROM public.partner_credential_events e WHERE e.tenant_id=p_tenant AND e.action='registered'
  AND NOT EXISTS(SELECT 1 FROM public.partner_credential_events r WHERE r.tenant_id=p_tenant AND r.credential_reference=e.credential_reference AND r.action='revoked') LOOP
  INSERT INTO public.partner_credential_events(id,tenant_id,credential_reference,action,audience,scopes,expires_at,actor_subject_id,correlation_id,created_at)
  VALUES(gen_random_uuid(),p_tenant,credential.credential_reference,'revoked',credential.audience,credential.scopes,credential.expires_at,p_actor,p_correlation,p_at);
 END LOOP;
 DELETE FROM public.account_credit_permissions WHERE tenant_id=p_tenant;
 DELETE FROM public.account_owners WHERE tenant_id=p_tenant;
 DELETE FROM public.tenant_subject_roles WHERE tenant_id=p_tenant;
 UPDATE public.accounts SET status='closed',closed_at=p_at,version=version+1,updated_at=GREATEST(updated_at,p_at) WHERE tenant_id=p_tenant AND status<>'closed';
 INSERT INTO public.partner_provisioning_requests(id,tenant_id,actor_subject_id,correlation_id,configuration_fingerprint,status,currency,account_count,sanitized_details,created_at)
 VALUES(request_id,p_tenant,p_actor,p_correlation,request.configuration_fingerprint,'rolled_back',request.currency,request.account_count,details,p_at);
 INSERT INTO public.audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)
 VALUES(request_id,p_tenant,p_actor,'partner.provisioning_rolled_back','tenant',p_tenant::text,'succeeded',p_correlation,details,p_at);
END $$;
REVOKE ALL ON FUNCTION controlled_apply_partner_v1(uuid,text,uuid,text,text,timestamptz),controlled_rollback_partner_v1(uuid,text,uuid,timestamptz) FROM PUBLIC;
