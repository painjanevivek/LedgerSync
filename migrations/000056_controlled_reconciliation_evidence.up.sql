CREATE FUNCTION controlled_record_reconciliation_v1(p_id uuid,p_tenant uuid,p_actor text,p_checked bigint,p_postings bigint,
 p_correlation uuid,p_started timestamptz,p_completed timestamptz,p_details jsonb,p_watermark text,p_application text,p_schema text,p_mismatches jsonb)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE mismatch jsonb; account_id uuid; transfer_id uuid; status text; outcome text; mismatch_count integer; scope_actor text:=current_setting('app.actor_id',true);
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR (p_actor IS NOT NULL AND p_actor IS DISTINCT FROM scope_actor)
  OR (p_actor IS NULL AND scope_actor NOT LIKE 'system:%') THEN RAISE EXCEPTION 'reconciliation scope denied' USING ERRCODE='42501'; END IF;
 IF NOT (SELECT rolsuper FROM pg_roles WHERE rolname=session_user) AND pg_has_role(session_user,'ledgersync_scheduler','MEMBER')
  AND NOT public.scheduler_tenant_allowed(p_tenant) THEN RAISE EXCEPTION 'scheduler tenant denied' USING ERRCODE='42501'; END IF;
 IF p_id IS NULL OR p_correlation IS NULL OR p_started IS NULL OR p_completed IS NULL
  OR p_checked IS NULL OR p_checked<0 OR p_postings IS NULL OR p_postings<0
  OR jsonb_typeof(p_mismatches) IS DISTINCT FROM 'array' OR jsonb_array_length(p_mismatches)>100000 OR octet_length(p_mismatches::text)>67108864
  OR jsonb_typeof(p_details) IS DISTINCT FROM 'object' OR octet_length(p_details::text)>65536
  OR p_watermark IS NULL OR p_watermark IS DISTINCT FROM txid_current_snapshot()::text
  OR p_application IS NULL OR octet_length(p_application) NOT BETWEEN 1 AND 256 OR p_schema IS NULL OR octet_length(p_schema) NOT BETWEEN 1 AND 256 THEN
  RAISE EXCEPTION 'invalid reconciliation evidence' USING ERRCODE='22023';
 END IF;
 IF p_checked<>(SELECT count(*) FROM public.accounts WHERE tenant_id=p_tenant)
  OR p_postings<>(SELECT count(*) FROM public.ledger_postings p JOIN public.accounts a ON a.id=p.account_id AND a.tenant_id=p.tenant_id WHERE p.tenant_id=p_tenant) THEN
  RAISE EXCEPTION 'reconciliation scope count mismatch' USING ERRCODE='23514';
 END IF;
 mismatch_count:=jsonb_array_length(p_mismatches);
 IF p_checked=0 AND (mismatch_count<>1 OR p_mismatches->0->>'Classification' IS DISTINCT FROM 'scope_empty') THEN
  RAISE EXCEPTION 'empty reconciliation scope cannot pass' USING ERRCODE='23514';
 END IF;
 status:=CASE WHEN mismatch_count=0 THEN 'matched' ELSE 'mismatch' END;
 outcome:=CASE WHEN status='matched' THEN 'succeeded' ELSE 'failed' END;
 INSERT INTO public.reconciliation_runs(id,tenant_id,status,checked_account_count,mismatch_count,correlation_id,started_at,completed_at,details,scope,ledger_watermark,application_version,schema_version,posting_count)
 VALUES(p_id,p_tenant,status,p_checked,mismatch_count,p_correlation,p_started,p_completed,p_details,'tenant_all_accounts',p_watermark,p_application,p_schema,p_postings);
 FOR mismatch IN SELECT value FROM jsonb_array_elements(p_mismatches) LOOP
  account_id:=NULLIF(mismatch->>'AccountID','')::uuid; transfer_id:=NULLIF(mismatch->>'TransferID','')::uuid;
  IF (account_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM public.accounts WHERE tenant_id=p_tenant AND id=account_id))
   OR (transfer_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM public.transfers WHERE tenant_id=p_tenant AND id=transfer_id))
   OR (mismatch->>'Classification'='scope_empty' AND p_checked<>0) THEN RAISE EXCEPTION 'mismatch target scope denied' USING ERRCODE='42501'; END IF;
  INSERT INTO public.reconciliation_mismatches(id,run_id,tenant_id,account_id,transfer_id,classification,currency,expected_minor,observed_minor,observed_available_minor,balance_version,sanitized_details,created_at)
  VALUES(gen_random_uuid(),p_id,p_tenant,account_id,transfer_id,mismatch->>'Classification',NULLIF(mismatch->>'Currency',''),
   (mismatch->>'ExpectedMinor')::bigint,(mismatch->>'ObservedMinor')::bigint,(mismatch->>'ObservedAvailableMinor')::bigint,(mismatch->>'BalanceVersion')::bigint,
   COALESCE(mismatch->'Details','{}'::jsonb),p_completed);
 END LOOP;
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),p_tenant,p_actor,'reconciliation.completed','reconciliation_run',p_id::text,outcome,p_correlation,
  jsonb_build_object('status',status,'scope','tenant_all_accounts','checked_account_count',p_checked,'posting_count',p_postings,'mismatch_count',mismatch_count,'ledger_watermark',p_watermark),p_completed);
END $$;
REVOKE ALL ON FUNCTION controlled_record_reconciliation_v1(uuid,uuid,text,bigint,bigint,uuid,timestamptz,timestamptz,jsonb,text,text,text,jsonb) FROM PUBLIC;
