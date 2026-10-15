-- Scheduler authority is separate from financial command authority.
CREATE TABLE reconciliation_scheduler_tenants (
 workload_name name NOT NULL,
 tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 PRIMARY KEY(workload_name,tenant_id)
);
CREATE TABLE reconciliation_scheduler_leases (
 tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
 owner_id text NOT NULL CHECK(length(owner_id) BETWEEN 1 AND 128),
 generation bigint NOT NULL CHECK(generation>0),
 expires_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL
);
CREATE TABLE reconciliation_scheduled_jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 scheduled_for timestamptz NOT NULL,
 owner_id text,
 generation bigint,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','running','completed')),
 run_id uuid REFERENCES reconciliation_runs(id),
 outcome text CHECK(outcome IN ('matched','mismatch','failed')),
 started_at timestamptz,
 completed_at timestamptz,
 UNIQUE(tenant_id,scheduled_for)
);
CREATE INDEX reconciliation_scheduled_jobs_pending ON reconciliation_scheduled_jobs(tenant_id,scheduled_for) WHERE state<>'completed';

CREATE FUNCTION scheduler_tenant_allowed(p_tenant uuid) RETURNS boolean
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=public,pg_temp AS $$
 SELECT EXISTS(SELECT 1 FROM reconciliation_scheduler_tenants WHERE workload_name=session_user AND tenant_id=p_tenant)
$$;
CREATE FUNCTION acquire_reconciliation_scheduler(p_tenant uuid,p_owner text,p_duration_ms integer) RETURNS bigint
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE token bigint; now_at timestamptz:=clock_timestamp();
BEGIN
 IF NOT scheduler_tenant_allowed(p_tenant) THEN RAISE EXCEPTION 'scheduler tenant denied' USING ERRCODE='42501'; END IF;
 IF length(p_owner) NOT BETWEEN 1 AND 128 OR p_duration_ms NOT BETWEEN 100 AND 60000 THEN RAISE EXCEPTION 'invalid scheduler lease'; END IF;
 INSERT INTO reconciliation_scheduler_leases(tenant_id,owner_id,generation,expires_at,updated_at)
 VALUES(p_tenant,p_owner,1,now_at+make_interval(secs=>p_duration_ms::double precision/1000),now_at)
 ON CONFLICT(tenant_id) DO UPDATE SET owner_id=excluded.owner_id,
 generation=CASE WHEN reconciliation_scheduler_leases.expires_at<=now_at THEN reconciliation_scheduler_leases.generation+1 ELSE reconciliation_scheduler_leases.generation END,
 expires_at=excluded.expires_at,updated_at=now_at
 WHERE reconciliation_scheduler_leases.expires_at<=now_at OR reconciliation_scheduler_leases.owner_id=p_owner
 RETURNING generation INTO token;
 RETURN token;
END $$;
CREATE FUNCTION renew_reconciliation_scheduler(p_tenant uuid,p_owner text,p_generation bigint,p_duration_ms integer) RETURNS boolean
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT scheduler_tenant_allowed(p_tenant) OR p_duration_ms NOT BETWEEN 100 AND 60000 THEN RAISE EXCEPTION 'scheduler renewal denied' USING ERRCODE='42501'; END IF;
 UPDATE reconciliation_scheduler_leases SET expires_at=clock_timestamp()+make_interval(secs=>p_duration_ms::double precision/1000),updated_at=clock_timestamp()
 WHERE tenant_id=p_tenant AND owner_id=p_owner AND generation=p_generation AND expires_at>clock_timestamp();
 RETURN FOUND;
END $$;
CREATE FUNCTION release_reconciliation_scheduler(p_tenant uuid,p_owner text,p_generation bigint) RETURNS void
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT scheduler_tenant_allowed(p_tenant) THEN RAISE EXCEPTION 'scheduler tenant denied' USING ERRCODE='42501'; END IF;
 UPDATE reconciliation_scheduler_leases SET expires_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=p_tenant AND owner_id=p_owner AND generation=p_generation;
END $$;
CREATE FUNCTION claim_reconciliation_schedule(p_tenant uuid,p_owner text,p_generation bigint,p_interval_seconds integer) RETURNS uuid
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE job uuid;
BEGIN
 IF NOT scheduler_tenant_allowed(p_tenant) OR p_interval_seconds NOT BETWEEN 15 AND 86400 THEN RAISE EXCEPTION 'scheduler tenant or interval denied' USING ERRCODE='42501'; END IF;
 PERFORM 1 FROM reconciliation_scheduler_leases WHERE tenant_id=p_tenant AND owner_id=p_owner AND generation=p_generation AND expires_at>clock_timestamp() FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'stale scheduler fence' USING ERRCODE='55000'; END IF;
 INSERT INTO reconciliation_scheduled_jobs(tenant_id,scheduled_for) VALUES(p_tenant,to_timestamp(floor(extract(epoch FROM clock_timestamp())/p_interval_seconds)*p_interval_seconds)) ON CONFLICT(tenant_id,scheduled_for) DO NOTHING;
 SELECT id INTO job FROM reconciliation_scheduled_jobs WHERE tenant_id=p_tenant AND (state='pending' OR state='running' AND generation<>p_generation) ORDER BY scheduled_for LIMIT 1 FOR UPDATE SKIP LOCKED;
 IF job IS NOT NULL THEN UPDATE reconciliation_scheduled_jobs SET owner_id=p_owner,generation=p_generation,state='running',started_at=clock_timestamp() WHERE id=job; END IF;
 RETURN job;
END $$;
CREATE FUNCTION complete_reconciliation_schedule(p_tenant uuid,p_owner text,p_generation bigint,p_job uuid,p_run uuid,p_outcome text) RETURNS void
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT scheduler_tenant_allowed(p_tenant) THEN RAISE EXCEPTION 'scheduler tenant denied' USING ERRCODE='42501'; END IF;
 PERFORM 1 FROM reconciliation_scheduler_leases WHERE tenant_id=p_tenant AND owner_id=p_owner AND generation=p_generation AND expires_at>clock_timestamp() FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'stale scheduler fence' USING ERRCODE='55000'; END IF;
 PERFORM 1 FROM reconciliation_runs WHERE tenant_id=p_tenant AND id=p_run AND status=p_outcome;
 IF NOT FOUND THEN RAISE EXCEPTION 'scheduled result unavailable'; END IF;
 UPDATE reconciliation_scheduled_jobs SET state='completed',run_id=p_run,outcome=p_outcome,completed_at=clock_timestamp() WHERE id=p_job AND tenant_id=p_tenant AND owner_id=p_owner AND generation=p_generation AND state='running';
 IF NOT FOUND THEN RAISE EXCEPTION 'stale scheduled job' USING ERRCODE='55000'; END IF;
END $$;

-- Scoped read views let the existing reconciliation repository use its normal
-- queries. The scheduler cannot SELECT base financial tables.
CREATE SCHEMA ledgersync_scheduler;
REVOKE ALL ON SCHEMA ledgersync_scheduler FROM PUBLIC;
DO $$ DECLARE relation text; BEGIN
 FOREACH relation IN ARRAY ARRAY['tenants','accounts','transfers','journal_transactions','ledger_postings','outbox_events'] LOOP
  EXECUTE format('CREATE VIEW ledgersync_scheduler.%I WITH (security_barrier=true) AS SELECT source.* FROM public.%I source WHERE public.scheduler_tenant_allowed(%s)',relation,relation,CASE WHEN relation='tenants' THEN 'source.id' ELSE 'source.tenant_id' END);
 END LOOP;
 FOREACH relation IN ARRAY ARRAY['account_opening_balances','account_balance_projections'] LOOP
  EXECUTE format('CREATE VIEW ledgersync_scheduler.%I WITH (security_barrier=true) AS SELECT source.* FROM public.%I source JOIN public.accounts account ON account.id=source.account_id WHERE public.scheduler_tenant_allowed(account.tenant_id)',relation,relation);
 END LOOP;
END $$;
CREATE VIEW ledgersync_scheduler.schema_migrations AS SELECT * FROM public.schema_migrations;

CREATE FUNCTION restrict_scheduler_evidence_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM pg_roles member_role JOIN pg_roles scheduler_role ON scheduler_role.rolname='ledgersync_scheduler' WHERE member_role.rolname=session_user AND NOT member_role.rolsuper AND pg_has_role(member_role.oid,scheduler_role.oid,'MEMBER')) THEN
  IF NOT public.scheduler_tenant_allowed(NEW.tenant_id) THEN RAISE EXCEPTION 'scheduler tenant denied' USING ERRCODE='42501'; END IF;
  IF TG_TABLE_NAME='audit_events' AND to_jsonb(NEW)->>'event_type'<>'reconciliation.completed' THEN RAISE EXCEPTION 'scheduler audit action denied' USING ERRCODE='42501'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER restrict_scheduler_runs BEFORE INSERT ON reconciliation_runs FOR EACH ROW EXECUTE FUNCTION restrict_scheduler_evidence_insert();
CREATE TRIGGER restrict_scheduler_mismatches BEFORE INSERT ON reconciliation_mismatches FOR EACH ROW EXECUTE FUNCTION restrict_scheduler_evidence_insert();
CREATE TRIGGER restrict_scheduler_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION restrict_scheduler_evidence_insert();
REVOKE ALL ON FUNCTION scheduler_tenant_allowed(uuid),acquire_reconciliation_scheduler(uuid,text,integer),renew_reconciliation_scheduler(uuid,text,bigint,integer),release_reconciliation_scheduler(uuid,text,bigint),claim_reconciliation_schedule(uuid,text,bigint,integer),complete_reconciliation_schedule(uuid,text,bigint,uuid,uuid,text) FROM PUBLIC;
