-- Historical/manual evidence import is administrative. Runtime workers use the
-- fenced completion function. No workload receives EXECUTE on this function.
CREATE FUNCTION controlled_import_delivery_evidence_v1(p_id uuid,p_tenant uuid,p_transfer uuid,p_outbox uuid,p_kind text,p_endpoint text,p_attempt integer,p_status text,p_response text,p_code text,p_due timestamptz,p_started timestamptz,p_completed timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR current_setting('app.actor_id',true) IS DISTINCT FROM 'system:delivery' THEN
  RAISE EXCEPTION 'delivery import scope denied' USING ERRCODE='42501';
 END IF;
 IF p_id IS NULL OR p_transfer IS NULL OR p_kind IS NULL OR p_kind NOT IN ('webhook','notification')
  OR p_endpoint IS NULL OR octet_length(p_endpoint) NOT BETWEEN 1 AND 256 OR p_attempt IS NULL OR p_attempt<1
  OR p_status IS NULL OR p_status NOT IN ('pending','retrying','delivered','dead') OR p_due IS NULL
  OR octet_length(p_response)>64 OR octet_length(p_code)>128 THEN RAISE EXCEPTION 'invalid delivery import' USING ERRCODE='22023'; END IF;
 IF NOT EXISTS(SELECT 1 FROM public.transfers WHERE tenant_id=p_tenant AND id=p_transfer)
  OR (p_outbox IS NOT NULL AND NOT EXISTS(SELECT 1 FROM public.outbox_events WHERE tenant_id=p_tenant AND id=p_outbox AND transfer_id=p_transfer)) THEN
  RAISE EXCEPTION 'delivery import source denied' USING ERRCODE='42501';
 END IF;
 INSERT INTO public.delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,response_class,sanitized_error_code,due_at,started_at,completed_at)
 VALUES(p_id,p_tenant,p_transfer,p_outbox,p_kind,p_endpoint,p_attempt,p_status,p_response,p_code,p_due,p_started,p_completed);
END $$;
REVOKE ALL ON FUNCTION controlled_import_delivery_evidence_v1(uuid,uuid,uuid,uuid,text,text,integer,text,text,text,timestamptz,timestamptz,timestamptz) FROM PUBLIC;
