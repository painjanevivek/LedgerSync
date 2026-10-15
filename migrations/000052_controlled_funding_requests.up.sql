CREATE FUNCTION controlled_fingerprint_parts_v1(p_parts text[]) RETURNS bytea LANGUAGE sql IMMUTABLE STRICT
SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT public.digest(decode(string_agg(encode(convert_to(value,'UTF8'),'hex'),'00' ORDER BY ordinal),'hex'),'sha256')
 FROM unnest(p_parts) WITH ORDINALITY AS part(value,ordinal)
$$;
REVOKE ALL ON FUNCTION controlled_fingerprint_parts_v1(text[]) FROM PUBLIC;

CREATE FUNCTION controlled_request_funding_v1(p_tenant uuid,p_actor text,p_destination uuid,p_currency text,p_amount bigint,
 p_external text,p_evidence text,p_key text,p_fingerprint bytea,p_correlation uuid,p_at timestamptz)
RETURNS TABLE(event_id uuid,replayed boolean) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE prior public.funding_events%ROWTYPE; policy record; clearing uuid; created uuid:=gen_random_uuid();
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role='finance') THEN
  RAISE EXCEPTION 'funding actor denied' USING ERRCODE='42501',CONSTRAINT='controlled_funding_actor';
 END IF;
 IF p_destination IS NULL OR p_amount IS NULL OR p_amount<=0 OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
  OR p_key IS NULL OR char_length(p_key) NOT BETWEEN 16 AND 255 OR p_correlation IS NULL OR p_at IS NULL
  OR p_external IS NULL OR char_length(p_external) NOT BETWEEN 1 AND 256 OR p_evidence IS NULL OR char_length(p_evidence) NOT BETWEEN 1 AND 512
  OR p_fingerprint IS DISTINCT FROM public.controlled_fingerprint_parts_v1(ARRAY[p_tenant::text,p_actor,p_destination::text,p_currency,p_amount::text,p_external,p_evidence]) THEN
  RAISE EXCEPTION 'invalid funding request' USING ERRCODE='22023',CONSTRAINT='controlled_funding_input';
 END IF;
 SELECT * INTO prior FROM public.funding_events WHERE tenant_id=p_tenant AND requester_subject_id=p_actor AND idempotency_key=p_key;
 IF FOUND THEN
  IF prior.request_fingerprint<>p_fingerprint THEN RAISE EXCEPTION 'funding idempotency conflict' USING ERRCODE='23505',CONSTRAINT='controlled_funding_idempotency'; END IF;
  RETURN QUERY SELECT prior.id,true; RETURN;
 END IF;
 SELECT * INTO policy FROM public.controlled_lock_funding_policy_v1(p_tenant,p_destination,p_currency);
 IF NOT FOUND THEN RAISE EXCEPTION 'funding destination not found' USING ERRCODE='P0002',CONSTRAINT='controlled_funding_not_found'; END IF;
 IF policy.mode='production_dual_control' AND NOT policy.finance_activated THEN RAISE EXCEPTION 'funding inactive' USING ERRCODE='42501',CONSTRAINT='controlled_funding_forbidden'; END IF;
 IF p_amount>policy.per_command_minor THEN RAISE EXCEPTION 'funding limit exceeded' USING ERRCODE='23514',CONSTRAINT='controlled_funding_limit'; END IF;
 clearing:=public.controlled_ensure_funding_account_v1(p_tenant,p_actor,p_currency,p_correlation,p_at);
 INSERT INTO public.funding_events(id,tenant_id,requester_subject_id,destination_account_id,system_account_id,external_reference,evidence_reference,
  idempotency_key,request_fingerprint,amount_minor,currency,status,demo_policy,policy_version,correlation_id,requested_at,updated_at)
 VALUES(created,p_tenant,p_actor,p_destination,clearing,p_external,p_evidence,p_key,p_fingerprint,p_amount,p_currency,'requested',false,policy.policy_version,p_correlation,p_at,p_at);
 INSERT INTO public.approval_records(id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,correlation_id,policy_version,created_at)
 VALUES(gen_random_uuid(),p_tenant,'funding',created,p_actor,'requested',p_at+interval '24 hours',p_correlation,policy.policy_version,p_at);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),p_tenant,p_actor,'funding.requested','funding_event',created::text,'succeeded',p_correlation,
  jsonb_build_object('funding_event_id',created::text,'terminology','recorded external value evidence'),p_at);
 RETURN QUERY SELECT created,false;
END $$;

CREATE FUNCTION controlled_request_funding_compensation_v1(p_tenant uuid,p_actor text,p_original uuid,p_reason text,p_note text,
 p_key text,p_fingerprint bytea,p_correlation uuid,p_at timestamptz)
RETURNS TABLE(event_id uuid,replayed boolean) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE prior public.funding_events%ROWTYPE; original public.funding_events%ROWTYPE; created uuid:=gen_random_uuid();
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role='finance') THEN
  RAISE EXCEPTION 'funding actor denied' USING ERRCODE='42501',CONSTRAINT='controlled_funding_actor';
 END IF;
 IF p_original IS NULL OR p_key IS NULL OR char_length(p_key) NOT BETWEEN 16 AND 255 OR p_correlation IS NULL OR p_at IS NULL
  OR p_reason IS NULL OR char_length(p_reason) NOT BETWEEN 1 AND 64 OR p_note IS NULL OR char_length(p_note) NOT BETWEEN 1 AND 500
  OR p_fingerprint IS DISTINCT FROM public.controlled_fingerprint_parts_v1(ARRAY[p_tenant::text,p_actor,p_original::text,p_reason,p_note]) THEN
  RAISE EXCEPTION 'invalid funding compensation request' USING ERRCODE='22023',CONSTRAINT='controlled_funding_input';
 END IF;
 SELECT * INTO prior FROM public.funding_events WHERE tenant_id=p_tenant AND requester_subject_id=p_actor AND idempotency_key=p_key;
 IF FOUND THEN
  IF prior.request_fingerprint<>p_fingerprint THEN RAISE EXCEPTION 'funding idempotency conflict' USING ERRCODE='23505',CONSTRAINT='controlled_funding_idempotency'; END IF;
  RETURN QUERY SELECT prior.id,true; RETURN;
 END IF;
 SELECT * INTO original FROM public.funding_events WHERE tenant_id=p_tenant AND id=p_original AND compensation_of_event_id IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'funding original not found' USING ERRCODE='P0002',CONSTRAINT='controlled_funding_not_found'; END IF;
 IF original.status<>'posted' OR original.compensation_event_id IS NOT NULL THEN RAISE EXCEPTION 'funding compensation conflict' USING ERRCODE='23505',CONSTRAINT='controlled_funding_idempotency'; END IF;
 IF NOT EXISTS(SELECT 1 FROM public.tenant_funding_policies WHERE tenant_id=p_tenant AND currency=original.currency AND (mode='local_demo_single_operator' OR finance_activated)) THEN
  RAISE EXCEPTION 'funding inactive' USING ERRCODE='42501',CONSTRAINT='controlled_funding_forbidden';
 END IF;
 INSERT INTO public.funding_events(id,tenant_id,requester_subject_id,destination_account_id,system_account_id,external_reference,evidence_reference,
  idempotency_key,request_fingerprint,amount_minor,currency,status,demo_policy,policy_version,compensation_of_event_id,compensation_reason_code,compensation_operator_note,correlation_id,requested_at,updated_at)
 VALUES(created,p_tenant,p_actor,original.destination_account_id,original.system_account_id,'compensation:'||p_original::text,original.evidence_reference,p_key,p_fingerprint,
  original.amount_minor,original.currency,'requested',false,original.policy_version,p_original,p_reason,p_note,p_correlation,p_at,p_at);
 INSERT INTO public.approval_records(id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,correlation_id,policy_version,created_at)
 VALUES(gen_random_uuid(),p_tenant,'funding_compensation',created,p_actor,'requested',p_at+interval '24 hours',p_correlation,original.policy_version,p_at);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),p_tenant,p_actor,'funding.compensation.requested','funding_event',created::text,'succeeded',p_correlation,
  jsonb_build_object('funding_event_id',created::text,'terminology','recorded external value evidence'),p_at);
 RETURN QUERY SELECT created,false;
END $$;

CREATE FUNCTION controlled_decide_funding_v1(p_tenant uuid,p_actor text,p_event uuid,p_decision text,p_demo boolean,p_reason text,p_correlation uuid,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE f public.funding_events%ROWTYPE; policy public.tenant_funding_policies%ROWTYPE; command_type text;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role='finance') THEN
  RAISE EXCEPTION 'funding actor denied' USING ERRCODE='42501',CONSTRAINT='controlled_funding_actor';
 END IF;
 IF p_event IS NULL OR p_decision IS NULL OR p_decision NOT IN ('approved','rejected') OR p_reason IS NULL OR char_length(p_reason) NOT BETWEEN 1 AND 500
  OR p_correlation IS NULL OR p_at IS NULL THEN RAISE EXCEPTION 'invalid funding decision' USING ERRCODE='22023',CONSTRAINT='controlled_funding_input'; END IF;
 SELECT * INTO f FROM public.funding_events WHERE tenant_id=p_tenant AND id=p_event FOR UPDATE;
 IF NOT FOUND THEN
  IF p_decision='rejected' THEN RAISE EXCEPTION 'funding decision conflict' USING ERRCODE='23505',CONSTRAINT='controlled_funding_idempotency'; END IF;
  RAISE EXCEPTION 'funding event not found' USING ERRCODE='P0002',CONSTRAINT='controlled_funding_not_found';
 END IF;
 SELECT * INTO STRICT policy FROM public.tenant_funding_policies WHERE tenant_id=p_tenant AND currency=f.currency;
 IF p_decision='approved' THEN
  IF p_demo IS DISTINCT FROM (policy.mode='local_demo_single_operator')
   OR (policy.mode='production_dual_control' AND (NOT policy.finance_activated OR f.requester_subject_id=p_actor)) THEN
   RAISE EXCEPTION 'funding independent approval required' USING ERRCODE='42501',CONSTRAINT='controlled_funding_forbidden';
  END IF;
  IF f.status='approved' THEN RETURN; END IF;
 END IF;
 IF f.status<>'requested' THEN RAISE EXCEPTION 'funding decision conflict' USING ERRCODE='23505',CONSTRAINT='controlled_funding_idempotency'; END IF;
 IF p_decision='approved' THEN
  UPDATE public.funding_events SET status='approved',approver_subject_id=p_actor,decision_reason=p_reason,demo_policy=p_demo,approved_at=p_at,updated_at=p_at WHERE tenant_id=p_tenant AND id=p_event;
 ELSE
  UPDATE public.funding_events SET status='rejected',approver_subject_id=p_actor,decision_reason=p_reason,rejected_at=p_at,updated_at=p_at WHERE tenant_id=p_tenant AND id=p_event;
 END IF;
 command_type:=CASE WHEN f.compensation_of_event_id IS NULL THEN 'funding' ELSE 'funding_compensation' END;
 INSERT INTO public.approval_records(id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at)
 VALUES(gen_random_uuid(),p_tenant,command_type,p_event,f.requester_subject_id,p_actor,p_decision,
  CASE WHEN p_decision='approved' THEN p_at+interval '24 hours' ELSE p_at END,p_reason,p_correlation,
  CASE WHEN p_decision='approved' THEN policy.policy_version ELSE f.policy_version END,p_at,p_at);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),p_tenant,p_actor,'funding.'||p_decision,'funding_event',p_event::text,'succeeded',p_correlation,
  jsonb_build_object('funding_event_id',p_event::text,'terminology','recorded external value evidence'),p_at);
END $$;
REVOKE ALL ON FUNCTION controlled_request_funding_v1(uuid,text,uuid,text,bigint,text,text,text,bytea,uuid,timestamptz),
 controlled_request_funding_compensation_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz),
 controlled_decide_funding_v1(uuid,text,uuid,text,boolean,text,uuid,timestamptz) FROM PUBLIC;
