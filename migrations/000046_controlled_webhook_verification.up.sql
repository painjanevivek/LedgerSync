-- Only a current fenced verification claim can activate its own endpoint.
-- The worker reports the result of its authenticated challenge protocol.
CREATE FUNCTION controlled_complete_webhook_verification_v1(
 p_job uuid,p_owner text,p_generation bigint,p_status text,p_completed timestamptz,
 p_retry timestamptz,p_error text,p_response text
) RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE item record; affected integer;
BEGIN
 IF p_status IS NULL OR p_status NOT IN ('verified','retrying','dead') OR p_completed IS NULL
    OR (p_status='retrying' AND p_retry IS NULL) OR length(COALESCE(p_error,''))>256 OR length(COALESCE(p_response,''))>256 THEN
  RAISE EXCEPTION 'invalid verification completion' USING ERRCODE='22023';
 END IF;
 SELECT j.tenant_id,j.webhook_id,j.actor_subject_id,j.correlation_id,j.expires_at,e.version INTO item
 FROM public.webhook_endpoint_verification_jobs j
 JOIN public.developer_webhook_endpoints e ON e.id=j.webhook_id AND e.tenant_id=j.tenant_id
 WHERE j.id=p_job AND j.claim_owner=p_owner AND j.claim_generation=p_generation
 AND j.claimed_until>clock_timestamp() AND j.status IN ('pending','retrying')
 AND public.tenant_scope_allows(j.tenant_id) AND current_setting('app.actor_id',true)=p_owner
 FOR UPDATE OF j,e;
 IF NOT FOUND THEN RETURN false; END IF;
 IF p_status='verified' THEN
  IF item.expires_at<=p_completed THEN RAISE EXCEPTION 'expired challenge cannot verify a webhook' USING ERRCODE='55000'; END IF;
  UPDATE public.developer_webhook_endpoints SET status='active',version=version+1,
   challenge_digest=NULL,challenge_expires_at=NULL,verified_at=p_completed,updated_at=p_completed
   WHERE id=item.webhook_id AND tenant_id=item.tenant_id AND status='pending_verification';
  GET DIAGNOSTICS affected=ROW_COUNT;
  IF affected<>1 THEN RAISE EXCEPTION 'webhook endpoint could not be activated' USING ERRCODE='55000'; END IF;
  INSERT INTO public.developer_webhook_events(id,tenant_id,webhook_id,action,version,actor_subject_id,correlation_id,sanitized_details,created_at)
  VALUES(gen_random_uuid(),item.tenant_id,item.webhook_id,'verified',item.version+1,item.actor_subject_id,item.correlation_id,
   jsonb_build_object('verification_job_id',p_job,'response_class',p_response),p_completed);
 END IF;
 UPDATE public.webhook_endpoint_verification_jobs SET status=p_status,
  attempt_number=CASE WHEN p_status='retrying' THEN attempt_number+1 ELSE attempt_number END,
  available_at=COALESCE(p_retry,available_at),claim_owner=NULL,claimed_until=NULL,last_error_code=NULLIF(p_error,''),
  completed_at=CASE WHEN p_status IN ('verified','dead') THEN p_completed ELSE NULL END,updated_at=p_completed
 WHERE id=p_job AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp();
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'stale verification completion' USING ERRCODE='55000'; END IF;
 RETURN true;
END $$;
REVOKE ALL ON FUNCTION controlled_complete_webhook_verification_v1(uuid,text,bigint,text,timestamptz,timestamptz,text,text) FROM PUBLIC;
