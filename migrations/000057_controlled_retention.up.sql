DO $$ BEGIN CREATE ROLE ledgersync_maintenance_owner NOLOGIN NOSUPERUSER NOBYPASSRLS; EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE FUNCTION controlled_run_retention_v1(p_tenant uuid,p_apply boolean,p_batch integer,p_outbox_seconds double precision,p_rate_seconds double precision,p_correlation uuid,p_application text,p_now timestamptz)
RETURNS TABLE(run_id uuid,mode text,published_outbox bigint,retained_idempotency bigint,expired_rates bigint,started_at timestamptz,completed_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE result_id uuid:=gen_random_uuid(); run_mode text; outbox_count bigint:=0; rate_count bigint:=0; retained_count bigint:=0;
BEGIN
 IF public.tenant_scope_allows(p_tenant) IS DISTINCT FROM true OR current_setting('app.actor_id',true) IS DISTINCT FROM 'system:retention' THEN
  RAISE EXCEPTION 'retention scope denied' USING ERRCODE='42501';
 END IF;
 IF p_apply IS NULL OR p_batch IS NULL OR p_batch NOT BETWEEN 1 AND 10000 OR p_now IS NULL OR p_correlation IS NULL
  OR p_application IS NULL OR octet_length(p_application) NOT BETWEEN 1 AND 256
  OR p_outbox_seconds IS NULL OR p_outbox_seconds<86400 OR p_outbox_seconds='NaN'::double precision
  OR p_rate_seconds IS NULL OR p_rate_seconds<3600 OR p_rate_seconds='NaN'::double precision THEN
  RAISE EXCEPTION 'invalid retention policy' USING ERRCODE='22023';
 END IF;
 run_mode:=CASE WHEN p_apply THEN 'apply' ELSE 'dry_run' END;
 IF p_apply THEN
  DELETE FROM public.outbox_events WHERE id IN (
   SELECT o.id FROM public.outbox_events o WHERE o.tenant_id=p_tenant AND o.published_at<p_now-p_outbox_seconds*interval '1 second' AND o.dead_at IS NULL
    AND NOT EXISTS(SELECT 1 FROM public.delivery_attempts d WHERE d.outbox_event_id=o.id)
    AND NOT EXISTS(SELECT 1 FROM public.webhook_delivery_jobs j WHERE j.outbox_event_id=o.id)
    AND NOT EXISTS(SELECT 1 FROM public.outbox_replay_actions a WHERE a.event_id=o.id)
    AND NOT EXISTS(SELECT 1 FROM public.outbox_publication_attempts a WHERE a.event_id=o.id)
    AND NOT EXISTS(SELECT 1 FROM public.logical_message_receipts r WHERE r.event_id=o.id)
   ORDER BY o.published_at,o.id LIMIT p_batch FOR UPDATE SKIP LOCKED);
  GET DIAGNOSTICS outbox_count=ROW_COUNT;
  DELETE FROM public.api_rate_limit_windows WHERE (tenant_id,principal_hash,route_key,window_started_at) IN (
   SELECT tenant_id,principal_hash,route_key,window_started_at FROM public.api_rate_limit_windows WHERE tenant_id=p_tenant AND window_started_at<p_now-p_rate_seconds*interval '1 second'
   ORDER BY window_started_at LIMIT p_batch FOR UPDATE SKIP LOCKED);
  GET DIAGNOSTICS rate_count=ROW_COUNT;
 ELSE
  SELECT count(*) INTO outbox_count FROM public.outbox_events o WHERE o.tenant_id=p_tenant AND o.published_at<p_now-p_outbox_seconds*interval '1 second' AND o.dead_at IS NULL
   AND NOT EXISTS(SELECT 1 FROM public.delivery_attempts d WHERE d.outbox_event_id=o.id)
   AND NOT EXISTS(SELECT 1 FROM public.webhook_delivery_jobs j WHERE j.outbox_event_id=o.id)
   AND NOT EXISTS(SELECT 1 FROM public.outbox_replay_actions a WHERE a.event_id=o.id)
   AND NOT EXISTS(SELECT 1 FROM public.outbox_publication_attempts a WHERE a.event_id=o.id)
   AND NOT EXISTS(SELECT 1 FROM public.logical_message_receipts r WHERE r.event_id=o.id);
  SELECT count(*) INTO rate_count FROM public.api_rate_limit_windows WHERE tenant_id=p_tenant AND window_started_at<p_now-p_rate_seconds*interval '1 second';
 END IF;
 SELECT count(*) INTO retained_count FROM public.idempotency_requests WHERE tenant_id=p_tenant AND state='completed' AND expires_at<p_now;
 INSERT INTO public.retention_runs(id,tenant_id,mode,published_outbox_count,retained_idempotency_count,expired_rate_window_count,correlation_id,application_version,started_at,completed_at)
 VALUES(result_id,p_tenant,run_mode,outbox_count,retained_count,rate_count,p_correlation,p_application,p_now,p_now);
 PERFORM public.controlled_append_audit_v1(result_id,p_tenant,NULL,'retention.completed','retention_run',result_id::text,'succeeded',p_correlation,
  jsonb_build_object('mode',run_mode,'published_outbox_count',outbox_count,'retained_idempotency_count',retained_count,'expired_rate_window_count',rate_count),p_now);
 RETURN QUERY SELECT result_id,run_mode,outbox_count,retained_count,rate_count,p_now,p_now;
END $$;
REVOKE ALL ON FUNCTION controlled_run_retention_v1(uuid,boolean,integer,double precision,double precision,uuid,text,timestamptz) FROM PUBLIC;
