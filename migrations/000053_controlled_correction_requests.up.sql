CREATE FUNCTION controlled_request_correction_v1(p_tenant uuid,p_actor text,p_original uuid,p_reason text,p_note text,
 p_key text,p_fingerprint bytea,p_correlation uuid,p_at timestamptz,p_step_up timestamptz)
RETURNS TABLE(correction_id uuid,replayed boolean) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE prior public.transfer_corrections%ROWTYPE; original record; created uuid:=gen_random_uuid(); expires timestamptz; recent boolean;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'correction actor denied' USING ERRCODE='42501',CONSTRAINT='controlled_correction_actor';
 END IF;
 IF p_original IS NULL OR p_reason IS NULL OR p_reason NOT IN ('duplicate','wrong_destination','wrong_amount','customer_request','operational_error','compliance_reversal')
  OR p_note IS NULL OR octet_length(p_note) NOT BETWEEN 1 AND 500 OR p_key IS NULL OR char_length(p_key) NOT BETWEEN 16 AND 255
  OR p_correlation IS NULL OR p_at IS NULL
  OR p_fingerprint IS DISTINCT FROM public.controlled_fingerprint_parts_v1(ARRAY[p_tenant::text,p_actor,p_original::text,p_reason,p_note]) THEN
  RAISE EXCEPTION 'invalid correction request' USING ERRCODE='22023',CONSTRAINT='controlled_correction_input';
 END IF;
 SELECT * INTO prior FROM public.transfer_corrections WHERE tenant_id=p_tenant AND requester_subject_id=p_actor AND idempotency_key=p_key;
 IF FOUND THEN
  IF prior.request_fingerprint<>p_fingerprint THEN RAISE EXCEPTION 'correction idempotency conflict' USING ERRCODE='23505',CONSTRAINT='controlled_correction_idempotency'; END IF;
  RETURN QUERY SELECT prior.id,true; RETURN;
 END IF;
 SELECT * INTO original FROM public.controlled_lock_correction_original_v1(p_tenant,p_original);
 IF NOT FOUND THEN RAISE EXCEPTION 'correction original not found' USING ERRCODE='P0002',CONSTRAINT='controlled_correction_not_found'; END IF;
 IF original.status<>'posted' OR original.compensation_of<>'' THEN RAISE EXCEPTION 'original cannot be corrected' USING ERRCODE='55000',CONSTRAINT='controlled_correction_conflict'; END IF;
 recent:=p_step_up IS NOT NULL AND p_step_up<=p_at+interval '1 minute' AND p_at-p_step_up<=interval '10 minutes';
 IF original.control_mode='production_dual_control' AND original.requires_step_up AND NOT recent THEN
  RAISE EXCEPTION 'recent step-up required' USING ERRCODE='42501',CONSTRAINT='controlled_correction_step_up';
 END IF;
 expires:=p_at+make_interval(mins=>original.approval_ttl_minutes);
 INSERT INTO public.transfer_corrections(id,tenant_id,original_transfer_id,requester_subject_id,reason_code,operator_note,idempotency_key,request_fingerprint,
  status,policy_version,control_mode,step_up_required,approval_expires_at,correlation_id,requested_at,updated_at)
 VALUES(created,p_tenant,p_original,p_actor,p_reason,p_note,p_key,p_fingerprint,'requested',original.policy_version,original.control_mode,original.requires_step_up,expires,p_correlation,p_at,p_at);
 INSERT INTO public.approval_records(id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,correlation_id,policy_version,created_at)
 VALUES(gen_random_uuid(),p_tenant,'transfer_compensation',created,p_actor,'requested',expires,p_correlation,original.policy_version,p_at);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),p_tenant,p_actor,'transfer_correction.requested','transfer_correction',created::text,'succeeded',p_correlation,
  jsonb_build_object('reason_code',p_reason,'step_up_verified',NOT original.requires_step_up OR recent),p_at);
 RETURN QUERY SELECT created,false;
END $$;

CREATE FUNCTION controlled_decide_correction_v1(p_tenant uuid,p_actor text,p_correction uuid,p_decision text,p_reason text,
 p_correlation uuid,p_at timestamptz,p_step_up timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE c public.transfer_corrections%ROWTYPE; decision text:=p_decision; reason text:=p_reason; approver text;
 outcome text:='succeeded'; expires timestamptz; recent boolean; metadata jsonb:='{}';
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true) THEN
  RAISE EXCEPTION 'correction scope denied' USING ERRCODE='42501',CONSTRAINT='controlled_correction_actor';
 END IF;
 IF p_correction IS NULL OR p_decision IS NULL OR p_decision NOT IN ('approved','rejected','cancelled')
  OR p_reason IS NULL OR octet_length(p_reason) NOT BETWEEN 1 AND 500 OR p_correlation IS NULL OR p_at IS NULL THEN
  RAISE EXCEPTION 'invalid correction decision' USING ERRCODE='22023',CONSTRAINT='controlled_correction_input';
 END IF;
 IF p_decision<>'cancelled' AND NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role='finance') THEN
  RAISE EXCEPTION 'correction finance actor required' USING ERRCODE='42501',CONSTRAINT='controlled_correction_actor';
 END IF;
 SELECT * INTO c FROM public.transfer_corrections WHERE tenant_id=p_tenant AND id=p_correction FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'correction not found' USING ERRCODE='P0002',CONSTRAINT='controlled_correction_not_found'; END IF;
 expires:=c.approval_expires_at;
 IF p_decision='cancelled' THEN
  IF c.requester_subject_id<>p_actor THEN RAISE EXCEPTION 'only requester can cancel' USING ERRCODE='42501',CONSTRAINT='controlled_correction_forbidden'; END IF;
  IF c.status NOT IN ('requested','approved') THEN RAISE EXCEPTION 'correction decision conflict' USING ERRCODE='55000',CONSTRAINT='controlled_correction_conflict'; END IF;
 ELSE
  IF p_decision='approved' AND c.status='approved' THEN RETURN; END IF;
  IF c.status<>'requested' THEN RAISE EXCEPTION 'correction decision conflict' USING ERRCODE='55000',CONSTRAINT='controlled_correction_conflict'; END IF;
  IF p_at>=expires THEN
   decision:='expired'; reason:='approval_window_expired'; outcome:='failed'; expires:=p_at;
  ELSE
   IF c.control_mode='production_dual_control' AND c.requester_subject_id=p_actor THEN
    RAISE EXCEPTION 'independent correction approver required' USING ERRCODE='42501',CONSTRAINT='controlled_correction_forbidden';
   END IF;
   recent:=p_step_up IS NOT NULL AND p_step_up<=p_at+interval '1 minute' AND p_at-p_step_up<=interval '10 minutes';
   IF c.control_mode='production_dual_control' AND c.step_up_required AND NOT recent THEN
    RAISE EXCEPTION 'recent step-up required' USING ERRCODE='42501',CONSTRAINT='controlled_correction_step_up';
   END IF;
   approver:=p_actor;
   metadata:=jsonb_build_object('step_up_verified',NOT c.step_up_required OR recent);
  END IF;
 END IF;
 IF decision IN ('cancelled','expired') THEN
  UPDATE public.transfer_corrections SET status=decision,decision_reason=reason,cancelled_at=p_at,updated_at=p_at WHERE tenant_id=p_tenant AND id=p_correction;
 ELSE
  UPDATE public.transfer_corrections SET status=decision,approver_subject_id=approver,decision_reason=reason,decided_at=p_at,updated_at=p_at WHERE tenant_id=p_tenant AND id=p_correction;
 END IF;
 INSERT INTO public.approval_records(id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at)
 VALUES(gen_random_uuid(),p_tenant,'transfer_compensation',p_correction,c.requester_subject_id,approver,decision,expires,reason,p_correlation,c.policy_version,p_at,p_at);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),p_tenant,p_actor,'transfer_correction.'||decision,'transfer_correction',p_correction::text,outcome,p_correlation,metadata,p_at);
END $$;
REVOKE ALL ON FUNCTION controlled_request_correction_v1(uuid,text,uuid,text,text,text,bytea,uuid,timestamptz,timestamptz),controlled_decide_correction_v1(uuid,text,uuid,text,text,uuid,timestamptz,timestamptz) FROM PUBLIC;
