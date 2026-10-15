-- Append metadata only. The transaction supplies tenant and actor authority.
CREATE FUNCTION controlled_append_audit_v1(p_id uuid,p_tenant uuid,p_actor text,p_event text,p_target_type text,
 p_target text,p_outcome text,p_correlation uuid,p_metadata jsonb,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE entry record; scope_actor text:=current_setting('app.actor_id',true);
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true
  OR (p_actor IS NOT NULL AND p_actor IS DISTINCT FROM scope_actor)
  OR (p_actor IS NULL AND scope_actor NOT LIKE 'system:%') THEN
  RAISE EXCEPTION 'audit scope denied' USING ERRCODE='42501';
 END IF;
 IF p_id IS NULL OR p_correlation IS NULL OR p_at IS NULL
  OR p_event IS NULL OR p_event!~'^[a-z][a-z0-9_.]{0,127}$'
  OR p_target_type IS NULL OR p_target_type!~'^[a-z][a-z0-9_]{0,63}$'
  OR (p_target IS NOT NULL AND (octet_length(p_target)>256 OR p_target~'[[:cntrl:]]'))
  OR p_outcome IS NULL OR p_outcome NOT IN ('allowed','denied','succeeded','failed')
  OR jsonb_typeof(p_metadata) IS DISTINCT FROM 'object' OR octet_length(p_metadata::text)>16384 THEN
  RAISE EXCEPTION 'invalid audit metadata envelope' USING ERRCODE='22023';
 END IF;
 FOR entry IN SELECT key,value FROM jsonb_each(p_metadata) LOOP
  IF entry.key!~'^[a-z][a-z0-9_]{0,63}$'
   OR entry.key~'(secret|token|authorization|cookie|session|csrf|consistency|credential|database_url|connection_string|dsn|private_key|api_key|access_key|amount|balance|email|phone|address|payload)'
   OR jsonb_typeof(entry.value) NOT IN ('string','number','boolean','null')
   OR char_length(entry.value#>>'{}')>256 OR (entry.value#>>'{}')~'[[:cntrl:]]' THEN
   RAISE EXCEPTION 'unsafe audit metadata' USING ERRCODE='22023';
  END IF;
 END LOOP;
 INSERT INTO public.audit_events(id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at)
 VALUES(p_id,p_tenant,p_actor,p_event,p_target_type,p_target,p_outcome,p_correlation,p_metadata,p_at);
END $$;
REVOKE ALL ON FUNCTION controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) FROM PUBLIC;
