-- Cross-tenant queue discovery has a separate, non-login function owner.
-- Workloads cannot become this role. Item processing remains tenant scoped.
DO $$ BEGIN CREATE ROLE ledgersync_queue_owner NOLOGIN NOSUPERUSER NOBYPASSRLS; EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE FUNCTION controlled_claim_outbox_v1(p_limit integer,p_owner text,p_seconds double precision)
RETURNS TABLE(event_id uuid,tenant_id uuid,transfer_id text,account_id text,aggregate_type text,aggregate_id uuid,event_type text,aggregate_version bigint,payload jsonb,occurred_at timestamptz,attempt_count integer,claim_generation bigint,node_id text,logical_timestamp bigint,parent_reference text) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $body$
#variable_conflict use_column
BEGIN
 IF p_owner IS NULL OR length(btrim(p_owner)) NOT BETWEEN 1 AND 256 OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 OR p_seconds IS NULL OR NOT(p_seconds>0 AND p_seconds<=600) THEN
  RAISE EXCEPTION 'invalid bounded queue claim' USING ERRCODE='22023';
 END IF;
 RETURN QUERY 
WITH candidates AS (
 SELECT id FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL
 AND available_at<=clock_timestamp() AND (claimed_until IS NULL OR claimed_until<=clock_timestamp())
 ORDER BY available_at,created_at,id LIMIT p_limit FOR UPDATE SKIP LOCKED
), claimed AS (
 UPDATE outbox_events e SET claim_owner=p_owner,claimed_until=clock_timestamp()+(p_seconds*interval '1 second'),
 claim_generation=e.claim_generation+1,attempt_count=e.attempt_count+1
 FROM candidates c WHERE e.id=c.id RETURNING e.*
), attempts AS (
 INSERT INTO outbox_publication_attempts(event_id,claim_generation,worker_id,status,started_at,lease_until)
 SELECT id,claim_generation,p_owner,'claimed',clock_timestamp(),claimed_until FROM claimed RETURNING event_id
)
SELECT e.id,e.tenant_id,COALESCE(e.transfer_id::text,''),COALESCE(e.account_id::text,''),
e.aggregate_type,e.aggregate_id,e.event_type,e.aggregate_version,e.payload,e.occurred_at,e.attempt_count,e.claim_generation,e.node_id,e.logical_timestamp,e.parent_reference
FROM claimed e JOIN attempts a ON a.event_id=e.id;
END $body$;
REVOKE ALL ON FUNCTION controlled_claim_outbox_v1(integer,text,double precision) FROM PUBLIC;

CREATE FUNCTION controlled_claim_webhook_delivery_v1(p_seconds double precision,p_limit integer,p_owner text)
RETURNS TABLE(job_id text,tenant_id text,transfer_id text,outbox_event_id text,webhook_id text,event_id text,event_type text,endpoint_url text,signing_key_reference text,signing_key_id text,payload text,attempt_number integer,started_at timestamptz,claim_generation bigint) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $body$
#variable_conflict use_column
BEGIN
 IF p_owner IS NULL OR length(btrim(p_owner)) NOT BETWEEN 1 AND 256 OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 OR p_seconds IS NULL OR NOT(p_seconds>0 AND p_seconds<=600) THEN
  RAISE EXCEPTION 'invalid bounded queue claim' USING ERRCODE='22023';
 END IF;
 RETURN QUERY 
WITH candidates AS (
  SELECT id
  FROM webhook_delivery_jobs
  WHERE status IN ('pending','retrying')
    AND available_at <= clock_timestamp()
    AND (claimed_until IS NULL OR claimed_until <= clock_timestamp())
    AND EXISTS (SELECT 1 FROM outbox_events event WHERE event.id=webhook_delivery_jobs.outbox_event_id AND event.published_at IS NOT NULL AND event.dead_at IS NULL)
  ORDER BY available_at,created_at,id
  LIMIT p_limit
  FOR UPDATE SKIP LOCKED
)
UPDATE webhook_delivery_jobs AS job
SET claim_owner=p_owner,claimed_until=clock_timestamp()+(p_seconds*interval '1 second'),started_at=clock_timestamp(),updated_at=clock_timestamp(),claim_generation=job.claim_generation+1
FROM candidates
WHERE job.id=candidates.id
RETURNING job.id::text,job.tenant_id::text,job.transfer_id::text,job.outbox_event_id::text,job.webhook_id::text,
  job.event_id::text,job.event_type,
  COALESCE((SELECT endpoint.endpoint_url FROM developer_webhook_endpoints endpoint WHERE endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id AND endpoint.status='active'),''),
  COALESCE((SELECT endpoint.signing_key_reference FROM developer_webhook_endpoints endpoint WHERE endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id AND endpoint.status='active'),''),
  COALESCE((SELECT endpoint.signing_key_id FROM developer_webhook_endpoints endpoint WHERE endpoint.id=job.webhook_id AND endpoint.tenant_id=job.tenant_id AND endpoint.status='active'),''),
  job.payload::text,job.attempt_number,job.started_at,job.claim_generation;
END $body$;
REVOKE ALL ON FUNCTION controlled_claim_webhook_delivery_v1(double precision,integer,text) FROM PUBLIC;

CREATE FUNCTION controlled_claim_webhook_verification_v1(p_seconds double precision,p_limit integer,p_owner text)
RETURNS TABLE(job_id text,tenant_id text,webhook_id text,endpoint_url text,signing_key_reference text,signing_key_id text,challenge bytea,expires_at timestamptz,attempt_number integer,claim_generation bigint) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $body$
#variable_conflict use_column
BEGIN
 IF p_owner IS NULL OR length(btrim(p_owner)) NOT BETWEEN 1 AND 256 OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 OR p_seconds IS NULL OR NOT(p_seconds>0 AND p_seconds<=600) THEN
  RAISE EXCEPTION 'invalid bounded queue claim' USING ERRCODE='22023';
 END IF;
 RETURN QUERY WITH candidates AS (
  SELECT id FROM webhook_endpoint_verification_jobs
  WHERE status IN ('pending','retrying') AND available_at <= clock_timestamp() AND (claimed_until IS NULL OR claimed_until <= clock_timestamp())
  ORDER BY available_at,created_at,id LIMIT p_limit FOR UPDATE SKIP LOCKED
), claimed AS (
  UPDATE webhook_endpoint_verification_jobs job SET claim_owner=p_owner,claimed_until=clock_timestamp()+(p_seconds*interval '1 second'),updated_at=clock_timestamp(),claim_generation=job.claim_generation+1
  FROM candidates WHERE job.id=candidates.id
  RETURNING job.id,job.tenant_id,job.webhook_id,job.challenge,job.expires_at,job.attempt_number,job.claim_generation
)
SELECT claimed.id::text,claimed.tenant_id::text,claimed.webhook_id::text,endpoint.endpoint_url,endpoint.signing_key_reference,endpoint.signing_key_id,claimed.challenge,claimed.expires_at,claimed.attempt_number,claimed.claim_generation
FROM claimed JOIN developer_webhook_endpoints endpoint ON endpoint.id=claimed.webhook_id AND endpoint.tenant_id=claimed.tenant_id
WHERE endpoint.status='pending_verification';
END $body$;
REVOKE ALL ON FUNCTION controlled_claim_webhook_verification_v1(double precision,integer,text) FROM PUBLIC;

-- Resolve only a currently owned, unexpired claim. The caller must recheck the
-- fence in its item transaction; resolution does not extend or transfer a lease.
CREATE FUNCTION controlled_claim_tenant_v1(p_queue text,p_id uuid,p_owner text,p_generation bigint) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE tenant uuid;
BEGIN
 IF p_owner IS NULL OR length(btrim(p_owner)) NOT BETWEEN 1 AND 256 OR p_generation IS NULL OR p_generation<1 THEN RETURN NULL; END IF;
 CASE p_queue
 WHEN 'outbox' THEN SELECT tenant_id INTO tenant FROM outbox_events WHERE id=p_id AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND published_at IS NULL AND dead_at IS NULL;
 WHEN 'webhook_delivery' THEN SELECT tenant_id INTO tenant FROM webhook_delivery_jobs WHERE id=p_id AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND status IN ('pending','retrying');
 WHEN 'webhook_verification' THEN SELECT tenant_id INTO tenant FROM webhook_endpoint_verification_jobs WHERE id=p_id AND claim_owner=p_owner AND claim_generation=p_generation AND claimed_until>clock_timestamp() AND status IN ('pending','retrying');
 ELSE RAISE EXCEPTION 'unknown queue' USING ERRCODE='22023';
 END CASE;
 RETURN tenant;
END $$;
REVOKE ALL ON FUNCTION controlled_claim_tenant_v1(text,uuid,text,bigint) FROM PUBLIC;
