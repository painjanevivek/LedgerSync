CREATE FUNCTION controlled_outbox_recovery_v1(p_tenant uuid,p_actor text,p_event uuid,p_action text,p_reason text,p_correlation uuid,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE approval public.outbox_replay_actions%ROWTYPE; action_id uuid:=gen_random_uuid(); inserted uuid;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'recovery actor denied' USING ERRCODE='42501',CONSTRAINT='controlled_recovery_not_approved';
 END IF;
 IF p_event IS NULL OR p_correlation IS NULL OR p_at IS NULL OR p_action IS NULL OR p_action NOT IN ('approve','execute') THEN
  RAISE EXCEPTION 'invalid recovery command' USING ERRCODE='22023';
 END IF;
 IF p_action='approve' THEN
  IF p_reason IS NULL OR p_reason!~'^[a-z0-9_]{3,64}$' THEN RAISE EXCEPTION 'invalid recovery reason' USING ERRCODE='22023'; END IF;
  IF NOT EXISTS(SELECT 1 FROM public.outbox_events WHERE tenant_id=p_tenant AND id=p_event AND dead_at IS NOT NULL AND published_at IS NULL) THEN
   RAISE EXCEPTION 'dead outbox event not found' USING ERRCODE='P0002',CONSTRAINT='controlled_recovery_outbox_not_found';
  END IF;
  INSERT INTO public.outbox_replay_actions(id,tenant_id,event_id,action,actor_subject_id,reason_code,correlation_id,created_at)
  VALUES(action_id,p_tenant,p_event,'approved',p_actor,p_reason,p_correlation,p_at)
  ON CONFLICT(event_id,action,correlation_id) DO NOTHING RETURNING id INTO inserted;
  IF inserted IS NULL THEN
   SELECT * INTO STRICT approval FROM public.outbox_replay_actions WHERE tenant_id=p_tenant AND event_id=p_event AND action='approved' AND correlation_id=p_correlation;
   IF approval.actor_subject_id<>p_actor OR approval.reason_code<>p_reason THEN RAISE EXCEPTION 'approval correlation conflict' USING ERRCODE='23505',CONSTRAINT='controlled_recovery_idempotency'; END IF;
   RETURN;
  END IF;
  PERFORM public.controlled_append_audit_v1(action_id,p_tenant,p_actor,'outbox.replay_approved','outbox_event',p_event::text,'succeeded',p_correlation,jsonb_build_object('reason_code',p_reason),p_at);
 ELSE
  SELECT * INTO approval FROM public.outbox_replay_actions WHERE tenant_id=p_tenant AND event_id=p_event AND action='approved' AND correlation_id=p_correlation FOR UPDATE;
  IF NOT FOUND OR p_at<approval.created_at OR p_at>=approval.created_at+interval '24 hours' THEN
   RAISE EXCEPTION 'live replay approval required' USING ERRCODE='42501',CONSTRAINT='controlled_recovery_not_approved';
  END IF;
  IF approval.actor_subject_id=p_actor THEN RAISE EXCEPTION 'independent replay actor required' USING ERRCODE='42501',CONSTRAINT='controlled_recovery_separation'; END IF;
  UPDATE public.outbox_events SET dead_at=NULL,last_error_code='approved_replay',attempt_count=0,available_at=p_at,claim_owner=NULL,claimed_until=NULL
  WHERE tenant_id=p_tenant AND id=p_event AND dead_at IS NOT NULL AND published_at IS NULL;
  IF NOT FOUND THEN RAISE EXCEPTION 'dead outbox event not found' USING ERRCODE='P0002',CONSTRAINT='controlled_recovery_outbox_not_found'; END IF;
  INSERT INTO public.outbox_replay_actions(id,tenant_id,event_id,action,actor_subject_id,reason_code,correlation_id,created_at)
  VALUES(action_id,p_tenant,p_event,'executed',p_actor,approval.reason_code,p_correlation,p_at);
  PERFORM public.controlled_append_audit_v1(action_id,p_tenant,p_actor,'outbox.replayed','outbox_event',p_event::text,'succeeded',p_correlation,
   jsonb_build_object('reason_code',approval.reason_code,'approved_by',approval.actor_subject_id),p_at);
 END IF;
END $$;

CREATE FUNCTION controlled_delivery_recovery_v1(p_tenant uuid,p_actor text,p_attempt uuid,p_action text,p_approval uuid,p_key text,p_reason text,p_correlation uuid,p_at timestamptz)
RETURNS TABLE(result_id uuid,replayed boolean) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE approval public.delivery_replay_actions%ROWTYPE; prior public.delivery_replay_actions%ROWTYPE; attempt public.delivery_attempts%ROWTYPE;
 action_id uuid:=gen_random_uuid(); inserted uuid; job_id uuid:=gen_random_uuid(); details jsonb;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR p_actor IS DISTINCT FROM current_setting('app.actor_id',true)
  OR NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=p_tenant AND subject_id=p_actor AND role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'recovery actor denied' USING ERRCODE='42501',CONSTRAINT='controlled_recovery_not_approved';
 END IF;
 IF p_attempt IS NULL OR p_correlation IS NULL OR p_at IS NULL OR p_action IS NULL OR p_action NOT IN ('approve','execute')
  OR p_key IS NULL OR p_key!~'^[!-~]{16,255}$' THEN RAISE EXCEPTION 'invalid delivery recovery command' USING ERRCODE='22023'; END IF;
 IF p_action='approve' THEN
  IF p_reason IS NULL OR p_reason!~'^[a-z0-9_]{3,64}$' THEN RAISE EXCEPTION 'invalid recovery reason' USING ERRCODE='22023'; END IF;
  IF NOT EXISTS(SELECT 1 FROM public.delivery_attempts WHERE tenant_id=p_tenant AND id=p_attempt AND status='dead') THEN
   RAISE EXCEPTION 'dead delivery attempt not found' USING ERRCODE='P0002',CONSTRAINT='controlled_recovery_delivery_not_found';
  END IF;
  INSERT INTO public.delivery_replay_actions(id,tenant_id,attempt_id,action,actor_subject_id,reason_code,correlation_id,request_key,created_at)
  VALUES(action_id,p_tenant,p_attempt,'approved',p_actor,p_reason,p_correlation,p_key,p_at)
  ON CONFLICT(tenant_id,attempt_id,action,request_key) DO NOTHING RETURNING id INTO inserted;
  IF inserted IS NULL THEN
   SELECT * INTO STRICT prior FROM public.delivery_replay_actions WHERE tenant_id=p_tenant AND attempt_id=p_attempt AND action='approved' AND request_key=p_key;
   IF prior.actor_subject_id<>p_actor OR prior.reason_code<>p_reason THEN RAISE EXCEPTION 'delivery approval key conflict' USING ERRCODE='23505',CONSTRAINT='controlled_recovery_idempotency'; END IF;
   RETURN QUERY SELECT prior.id,true; RETURN;
  END IF;
  PERFORM public.controlled_append_audit_v1(action_id,p_tenant,p_actor,'delivery.replay_approved','delivery_attempt',p_attempt::text,'succeeded',p_correlation,jsonb_build_object('reason_code',p_reason),p_at);
  RETURN QUERY SELECT action_id,false; RETURN;
 END IF;
 SELECT * INTO prior FROM public.delivery_replay_actions WHERE tenant_id=p_tenant AND attempt_id=p_attempt AND action='executed';
 IF FOUND THEN
  IF prior.request_key<>p_key OR prior.actor_subject_id<>p_actor OR prior.sanitized_details->>'approval_id' IS DISTINCT FROM p_approval::text THEN
   RAISE EXCEPTION 'delivery execution key conflict' USING ERRCODE='23505',CONSTRAINT='controlled_recovery_idempotency';
  END IF;
  RETURN QUERY SELECT (prior.sanitized_details->>'webhook_delivery_job_id')::uuid,true; RETURN;
 END IF;
 SELECT * INTO approval FROM public.delivery_replay_actions WHERE tenant_id=p_tenant AND id=p_approval AND attempt_id=p_attempt AND action='approved' FOR UPDATE;
 IF NOT FOUND OR p_at<approval.created_at OR p_at>=approval.created_at+interval '24 hours' THEN
  RAISE EXCEPTION 'live delivery replay approval required' USING ERRCODE='42501',CONSTRAINT='controlled_recovery_not_approved';
 END IF;
 IF approval.actor_subject_id=p_actor THEN RAISE EXCEPTION 'independent replay actor required' USING ERRCODE='42501',CONSTRAINT='controlled_recovery_separation'; END IF;
 SELECT * INTO attempt FROM public.delivery_attempts WHERE tenant_id=p_tenant AND id=p_attempt AND status='dead' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'dead delivery attempt not found' USING ERRCODE='P0002',CONSTRAINT='controlled_recovery_delivery_not_found'; END IF;
 IF attempt.attempt_number<>(SELECT max(attempt_number) FROM public.delivery_attempts WHERE tenant_id=p_tenant AND transfer_id=attempt.transfer_id AND delivery_kind=attempt.delivery_kind AND endpoint_reference=attempt.endpoint_reference)
  OR attempt.delivery_kind<>'webhook' OR attempt.outbox_event_id IS NULL THEN RAISE EXCEPTION 'delivery replay source conflict' USING ERRCODE='55000'; END IF;
 INSERT INTO public.webhook_delivery_jobs(id,tenant_id,transfer_id,outbox_event_id,webhook_id,event_id,event_type,payload,attempt_number,replay_of_attempt_id,available_at,created_at,updated_at)
 SELECT job_id,p_tenant,attempt.transfer_id,e.id,w.id,e.id,'transfer.posted',e.payload,attempt.attempt_number+1,p_attempt,p_at,p_at,p_at
 FROM public.outbox_events e JOIN public.developer_webhook_endpoints w ON w.tenant_id=e.tenant_id AND w.id::text=attempt.endpoint_reference
 WHERE e.tenant_id=p_tenant AND e.id=attempt.outbox_event_id AND e.transfer_id=attempt.transfer_id AND e.event_type='transfer.posted.v1';
 IF NOT FOUND THEN RAISE EXCEPTION 'delivery replay source unavailable' USING ERRCODE='55000'; END IF;
 details:=jsonb_build_object('approval_id',p_approval::text,'approved_by',approval.actor_subject_id,'webhook_delivery_job_id',job_id::text,'reason_code',approval.reason_code);
 INSERT INTO public.delivery_replay_actions(id,tenant_id,attempt_id,action,actor_subject_id,reason_code,correlation_id,request_key,sanitized_details,created_at)
 VALUES(action_id,p_tenant,p_attempt,'executed',p_actor,approval.reason_code,p_correlation,p_key,details,p_at);
 PERFORM public.controlled_append_audit_v1(action_id,p_tenant,p_actor,'delivery.replayed','delivery_attempt',p_attempt::text,'succeeded',p_correlation,details,p_at);
 RETURN QUERY SELECT job_id,false;
END $$;
REVOKE ALL ON FUNCTION controlled_outbox_recovery_v1(uuid,text,uuid,text,text,uuid,timestamptz),controlled_delivery_recovery_v1(uuid,text,uuid,text,uuid,text,text,uuid,timestamptz) FROM PUBLIC;
