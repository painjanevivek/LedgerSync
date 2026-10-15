CREATE FUNCTION controlled_renew_claim_v1(p_queue text,p_id uuid,p_owner text,p_generation bigint,p_seconds double precision)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE tenant uuid; renewed timestamptz;
BEGIN
 IF p_seconds IS NULL OR p_seconds<=0 OR p_seconds>600 OR p_seconds='NaN'::double precision THEN RAISE EXCEPTION 'invalid lease duration' USING ERRCODE='22023'; END IF;
 tenant:=public.controlled_claim_tenant_v1(p_queue,p_id,p_owner,p_generation);
 IF tenant IS NULL OR public.tenant_scope_allows(tenant) IS DISTINCT FROM true OR p_owner IS DISTINCT FROM current_setting('app.actor_id',true) THEN RETURN false; END IF;
 IF p_queue='outbox' THEN
  UPDATE public.outbox_events SET claimed_until=clock_timestamp()+p_seconds*interval '1 second'
  WHERE id=p_id AND tenant_id=tenant AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND published_at IS NULL AND dead_at IS NULL
  RETURNING claimed_until INTO renewed;
  IF NOT FOUND THEN RETURN false; END IF;
  UPDATE public.outbox_publication_attempts SET lease_until=renewed WHERE event_id=p_id AND claim_generation=p_generation AND worker_id=p_owner AND status='claimed';
  IF NOT FOUND THEN RAISE EXCEPTION 'missing live publication attempt' USING ERRCODE='23514'; END IF;
 ELSIF p_queue='webhook_delivery' THEN
  UPDATE public.webhook_delivery_jobs SET claimed_until=clock_timestamp()+p_seconds*interval '1 second'
  WHERE id=p_id AND tenant_id=tenant AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND status IN ('pending','retrying');
  IF NOT FOUND THEN RETURN false; END IF;
 ELSIF p_queue='webhook_verification' THEN
  UPDATE public.webhook_endpoint_verification_jobs SET claimed_until=clock_timestamp()+p_seconds*interval '1 second'
  WHERE id=p_id AND tenant_id=tenant AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND status IN ('pending','retrying');
  IF NOT FOUND THEN RETURN false; END IF;
 ELSE RAISE EXCEPTION 'unknown claim queue' USING ERRCODE='22023';
 END IF;
 RETURN true;
END $$;

CREATE FUNCTION controlled_complete_outbox_v1(p_id uuid,p_owner text,p_generation bigint,p_at timestamptz,p_status text,p_code text)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE tenant uuid;
BEGIN
 IF p_at IS NULL OR p_status IS NULL OR p_status NOT IN ('published','retrying','dead') OR p_code IS NULL OR octet_length(p_code)>128 OR p_code~'[[:cntrl:]]' THEN
  RAISE EXCEPTION 'invalid outbox completion' USING ERRCODE='22023';
 END IF;
 tenant:=public.controlled_claim_tenant_v1('outbox',p_id,p_owner,p_generation);
 IF tenant IS NULL OR public.tenant_scope_allows(tenant) IS DISTINCT FROM true OR p_owner IS DISTINCT FROM current_setting('app.actor_id',true) THEN RETURN false; END IF;
 UPDATE public.outbox_events SET published_at=CASE WHEN p_status='published' THEN clock_timestamp() ELSE published_at END,
  dead_at=CASE WHEN p_status='dead' THEN clock_timestamp() ELSE dead_at END,available_at=CASE WHEN p_status='retrying' THEN p_at ELSE available_at END,
  last_error_code=NULLIF(p_code,''),claim_owner=NULL,claimed_until=NULL
 WHERE id=p_id AND tenant_id=tenant AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND published_at IS NULL AND dead_at IS NULL;
 IF NOT FOUND THEN RETURN false; END IF;
 UPDATE public.outbox_publication_attempts SET status=p_status,error_code=NULLIF(p_code,''),completed_at=clock_timestamp()
 WHERE event_id=p_id AND claim_generation=p_generation AND worker_id=p_owner AND status='claimed';
 IF NOT FOUND THEN RAISE EXCEPTION 'missing live publication attempt' USING ERRCODE='23514'; END IF;
 RETURN true;
END $$;

CREATE FUNCTION controlled_complete_delivery_v1(p_id uuid,p_owner text,p_generation bigint,p_attempt integer,p_status text,p_at timestamptz,p_retry timestamptz,p_response text,p_code text)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE job public.webhook_delivery_jobs%ROWTYPE;
BEGIN
 IF p_at IS NULL OR p_attempt IS NULL OR p_attempt<1 OR p_status IS NULL OR p_status NOT IN ('delivered','retrying','dead')
  OR (p_status='retrying' AND p_retry IS NULL) OR p_response IS NULL OR octet_length(p_response)>64 OR p_response~'[[:cntrl:]]'
  OR p_code IS NULL OR octet_length(p_code)>128 OR p_code~'[[:cntrl:]]' THEN RAISE EXCEPTION 'invalid delivery completion' USING ERRCODE='22023'; END IF;
 SELECT * INTO job FROM public.webhook_delivery_jobs WHERE id=p_id AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND status IN ('pending','retrying') FOR UPDATE;
 IF NOT FOUND OR public.tenant_scope_allows(job.tenant_id) IS DISTINCT FROM true OR p_owner IS DISTINCT FROM current_setting('app.actor_id',true) OR p_attempt<>job.attempt_number THEN RETURN false; END IF;
 INSERT INTO public.delivery_attempts(id,tenant_id,transfer_id,outbox_event_id,delivery_kind,endpoint_reference,attempt_number,status,response_class,sanitized_error_code,due_at,started_at,completed_at)
 VALUES(gen_random_uuid(),job.tenant_id,job.transfer_id,job.outbox_event_id,'webhook',job.webhook_id::text,p_attempt,p_status,NULLIF(p_response,''),NULLIF(p_code,''),COALESCE(p_retry,p_at),job.started_at,p_at);
 UPDATE public.webhook_delivery_jobs SET status=p_status,attempt_number=CASE WHEN p_status='retrying' THEN attempt_number+1 ELSE attempt_number END,
  available_at=COALESCE(p_retry,available_at),claim_owner=NULL,claimed_until=NULL,last_error_code=NULLIF(p_code,''),completed_at=CASE WHEN p_status IN ('delivered','dead') THEN p_at ELSE NULL END,updated_at=p_at
 WHERE id=p_id AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp();
 IF NOT FOUND THEN RAISE EXCEPTION 'delivery lease expired during completion' USING ERRCODE='40001'; END IF;
 RETURN true;
END $$;

CREATE FUNCTION controlled_prepare_publication_v1(p_id uuid,p_owner text,p_generation bigint,p_node text)
RETURNS TABLE(node_id text,logical_timestamp bigint,parent_reference text,send_counter bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE event public.outbox_events%ROWTYPE; received bigint; sent bigint;
BEGIN
 SELECT * INTO event FROM public.outbox_events WHERE id=p_id AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND published_at IS NULL AND dead_at IS NULL FOR UPDATE;
 IF NOT FOUND OR public.tenant_scope_allows(event.tenant_id) IS DISTINCT FROM true OR p_owner IS DISTINCT FROM current_setting('app.actor_id',true) THEN RETURN; END IF;
 received:=public.advance_logical_clock(p_node,COALESCE(event.logical_timestamp,0));
 sent:=public.advance_logical_clock(p_node,0);
 UPDATE public.outbox_publication_attempts a SET node_id=p_node,receive_counter=received,send_counter=sent WHERE a.event_id=p_id AND a.claim_generation=p_generation AND a.worker_id=p_owner AND a.status='claimed';
 IF NOT FOUND THEN RAISE EXCEPTION 'missing live publication attempt' USING ERRCODE='23514'; END IF;
 RETURN QUERY SELECT event.node_id,event.logical_timestamp,event.parent_reference,sent;
END $$;

CREATE FUNCTION controlled_receive_message_v1(p_tenant uuid,p_node text,p_event uuid,p_transport text,p_sender text,p_counter bigint)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE received bigint;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR current_setting('app.actor_id',true) IS DISTINCT FROM 'system:'||p_node THEN
  RAISE EXCEPTION 'message receipt scope denied' USING ERRCODE='42501';
 END IF;
 IF p_event IS NULL OR p_transport IS NULL OR octet_length(p_transport) NOT BETWEEN 1 AND 128
  OR (p_sender IS NULL)<>(p_counter IS NULL) OR octet_length(p_sender) NOT BETWEEN 1 AND 128 OR p_counter<1 THEN
  RAISE EXCEPTION 'invalid message receipt' USING ERRCODE='22023';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM public.outbox_events WHERE id=p_event AND tenant_id=p_tenant) THEN RAISE EXCEPTION 'message event scope denied' USING ERRCODE='42501'; END IF;
 received:=public.advance_logical_clock(p_node,COALESCE(p_counter,0));
 INSERT INTO public.logical_message_receipts(node_id,receive_counter,tenant_id,event_id,transport_id,sender_node_id,sender_counter)
 VALUES(p_node,received,p_tenant,p_event,p_transport,p_sender,p_counter);
END $$;
REVOKE ALL ON FUNCTION controlled_renew_claim_v1(text,uuid,text,bigint,double precision),controlled_complete_outbox_v1(uuid,text,bigint,timestamptz,text,text),
 controlled_complete_delivery_v1(uuid,text,bigint,integer,text,timestamptz,timestamptz,text,text),controlled_prepare_publication_v1(uuid,text,bigint,text),
 controlled_receive_message_v1(uuid,text,uuid,text,text,bigint) FROM PUBLIC;
