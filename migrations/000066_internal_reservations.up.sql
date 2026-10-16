-- Internal full-value reservations. No historical financial row is rewritten.
CREATE TABLE account_holds (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id),
 debit_account_id uuid NOT NULL, credit_account_id uuid NOT NULL,
 amount_minor bigint NOT NULL CHECK(amount_minor>0), currency text NOT NULL CHECK(currency~'^[A-Z]{3}$'),
 state text NOT NULL CHECK(state IN ('active','captured','released','expired')),
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL,
 created_by text NOT NULL, version bigint NOT NULL DEFAULT 1 CHECK(version IN (1,2)),
 completed_at timestamptz, transfer_id uuid REFERENCES transfers(id),
 CHECK(debit_account_id<>credit_account_id), CHECK(expires_at>created_at),
 CHECK((state='active' AND version=1 AND completed_at IS NULL AND transfer_id IS NULL)
  OR (state='captured' AND version=2 AND completed_at IS NOT NULL AND transfer_id IS NOT NULL)
  OR (state IN ('released','expired') AND version=2 AND completed_at IS NOT NULL AND transfer_id IS NULL)),
 UNIQUE(id,tenant_id),
 FOREIGN KEY(debit_account_id,tenant_id) REFERENCES accounts(id,tenant_id),
 FOREIGN KEY(credit_account_id,tenant_id) REFERENCES accounts(id,tenant_id)
);
CREATE INDEX account_holds_active_idx ON account_holds(tenant_id,debit_account_id,id) WHERE state='active';
CREATE INDEX account_holds_recent_idx ON account_holds(tenant_id,created_at DESC,id DESC);
CREATE TABLE hold_commands (
 tenant_id uuid NOT NULL REFERENCES tenants(id), actor_subject_id text NOT NULL,
 operation text NOT NULL CHECK(operation IN ('create','capture','release')),
 idempotency_key text NOT NULL, fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32),
 response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,actor_subject_id,operation,idempotency_key)
);
CREATE TABLE hold_events (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, hold_id uuid NOT NULL,
 state text NOT NULL, actor_subject_id text NOT NULL, correlation_id uuid NOT NULL,
 source_balance_version bigint NOT NULL CHECK(source_balance_version>=0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(hold_id,tenant_id) REFERENCES account_holds(id,tenant_id), UNIQUE(hold_id,state)
);
CREATE TABLE hold_expiry_jobs (
 hold_id uuid PRIMARY KEY, tenant_id uuid NOT NULL, expires_at timestamptz NOT NULL,
 done boolean NOT NULL DEFAULT false, claim_owner text, claim_generation bigint NOT NULL DEFAULT 0 CHECK(claim_generation>=0),
 claimed_until timestamptz,
 FOREIGN KEY(hold_id,tenant_id) REFERENCES account_holds(id,tenant_id),
 CHECK((claim_owner IS NULL)=(claimed_until IS NULL))
);
CREATE INDEX hold_expiry_due_idx ON hold_expiry_jobs(expires_at,hold_id) WHERE NOT done;
DO $$ DECLARE relation text; BEGIN
 FOREACH relation IN ARRAY ARRAY['account_holds','hold_commands','hold_events','hold_expiry_jobs'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',relation);
  EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY',relation);
  EXECUTE format('CREATE POLICY tenant_scope ON public.%I USING(public.tenant_scope_allows(tenant_id)) WITH CHECK(public.tenant_scope_allows(tenant_id))',relation);
 END LOOP;
 FOREACH relation IN ARRAY ARRAY['hold_commands','hold_events'] LOOP
  EXECUTE format('CREATE TRIGGER hold_append_only BEFORE UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION public.reject_row_mutation()',relation);
 END LOOP;
END $$;
CREATE POLICY queue_discovery ON hold_expiry_jobs TO ledgersync_queue_owner USING(true) WITH CHECK(true);
CREATE FUNCTION protect_hold_history_v1() RETURNS trigger
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF TG_OP='DELETE' OR OLD.state<>'active' OR NEW.state='active' OR NEW.version<>2
  OR (to_jsonb(NEW)-ARRAY['state','version','completed_at','transfer_id']) IS DISTINCT FROM
     (to_jsonb(OLD)-ARRAY['state','version','completed_at','transfer_id']) THEN
  RAISE EXCEPTION 'hold history is immutable' USING ERRCODE='55000';
 END IF; RETURN NEW;
END $$;
CREATE TRIGGER hold_history BEFORE UPDATE OR DELETE ON account_holds FOR EACH ROW EXECUTE FUNCTION protect_hold_history_v1();

ALTER TABLE outbox_events ADD COLUMN hold_id uuid REFERENCES account_holds(id);
ALTER TABLE outbox_events DROP CONSTRAINT outbox_command_aggregate_consistency;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_command_aggregate_consistency CHECK (
 (aggregate_type='account_balance' AND account_id IS NOT NULL AND aggregate_id=account_id
  AND num_nonnulls(transfer_id,funding_event_id,opening_import_id,hold_id)=1)
 OR (aggregate_type='account' AND num_nonnulls(transfer_id,funding_event_id,opening_import_id,hold_id)=0 AND account_id IS NOT NULL AND aggregate_id=account_id)
 OR (aggregate_type='funding_event' AND funding_event_id IS NOT NULL AND num_nonnulls(transfer_id,opening_import_id,hold_id)=0 AND aggregate_id=funding_event_id)
 OR (aggregate_type='transfer' AND transfer_id IS NOT NULL AND num_nonnulls(funding_event_id,opening_import_id,hold_id)=0 AND account_id IS NULL AND aggregate_id=transfer_id)
 OR (aggregate_type NOT IN ('account_balance','account','funding_event','transfer') AND num_nonnulls(transfer_id,funding_event_id,opening_import_id,hold_id)=0)
);

-- Existing writers preserve reserved funds by changing ledger and available by
-- the same delta. Promote only events whose old v1 equality assumption is false.
CREATE FUNCTION enrich_reserved_balance_event_v1() RETURNS trigger
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE balance public.account_balance_projections%ROWTYPE; BEGIN
 IF NEW.event_type='account.balance.changed.v1' AND NEW.account_id IS NOT NULL THEN
  SELECT * INTO STRICT balance FROM public.account_balance_projections WHERE account_id=NEW.account_id;
  IF balance.ledger_minor<>balance.available_minor THEN
   NEW.event_type:='account.balance.changed.v2';
   NEW.payload:=NEW.payload||jsonb_build_object('event_type',NEW.event_type,'ledger_minor',balance.ledger_minor::text,
    'reserved_minor',(balance.ledger_minor::numeric-balance.available_minor::numeric)::text);
  END IF;
 END IF; RETURN NEW;
END $$;
CREATE TRIGGER enrich_reserved_balance BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION enrich_reserved_balance_event_v1();

-- Private helpers are invoker routines. Workload roles have no EXECUTE grant.
CREATE FUNCTION hold_command_existing_v1(t uuid,a text,op text,k text,c uuid,fp bytea) RETURNS jsonb
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ DECLARE previous record; BEGIN
 IF public.tenant_scope_allows(t) IS DISTINCT FROM true OR a IS DISTINCT FROM current_setting('app.actor_id',true) THEN
  RAISE EXCEPTION 'hold scope denied' USING ERRCODE='42501';
 END IF;
 IF c IS NULL OR op NOT IN ('create','capture','release') OR k IS NULL OR k!~'^[!-~]{16,255}$' OR octet_length(fp)<>32 THEN
  RAISE EXCEPTION 'invalid hold command' USING ERRCODE='22023';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=t AND subject_id=a AND role IN ('operator','finance')) THEN
  RAISE EXCEPTION 'hold actor denied' USING ERRCODE='42501';
 END IF;
 -- Share the policy sequence with ordinary transfers. Account row locks remain
 -- the authority even for callers that do not use this advisory sequence.
 PERFORM pg_advisory_xact_lock(hashtextextended('hold-command|'||t::text||'|'||a||'|'||op||'|'||k,0));
 SELECT fingerprint,response INTO previous FROM public.hold_commands
  WHERE tenant_id=t AND actor_subject_id=a AND operation=op AND idempotency_key=k;
 IF FOUND THEN
  IF previous.fingerprint<>fp THEN RAISE EXCEPTION 'hold intent conflict' USING ERRCODE='23505',CONSTRAINT='hold_intent_conflict'; END IF;
  RETURN previous.response||jsonb_build_object('replayed',true);
 END IF; RETURN NULL;
END $$;
CREATE FUNCTION hold_command_store_v1(t uuid,a text,op text,k text,fp bytea,result jsonb) RETURNS jsonb
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 INSERT INTO public.hold_commands(tenant_id,actor_subject_id,operation,idempotency_key,fingerprint,response)
  VALUES(t,a,op,k,fp,result);
 RETURN result;
END $$;
CREATE FUNCTION hold_record_change_v1(t uuid,a text,h uuid,c uuid,new_state text,emit_balance boolean) RETURNS bigint
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE source uuid; currency_code text; balance public.account_balance_projections%ROWTYPE; event uuid:=gen_random_uuid(); BEGIN
 SELECT debit_account_id,currency INTO STRICT source,currency_code FROM public.account_holds WHERE id=h AND tenant_id=t;
 SELECT * INTO STRICT balance FROM public.account_balance_projections WHERE account_id=source;
 INSERT INTO public.hold_events(id,tenant_id,hold_id,state,actor_subject_id,correlation_id,source_balance_version)
  VALUES(gen_random_uuid(),t,h,new_state,a,c,balance.balance_version);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),t,a,'hold.'||new_state,'hold',h::text,'succeeded',c,
  jsonb_build_object('state',new_state),clock_timestamp());
 IF emit_balance THEN
  INSERT INTO public.outbox_events(id,tenant_id,hold_id,account_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at)
   VALUES(event,t,h,source,'account_balance',source,'account.balance.changed.v2',balance.balance_version,
    jsonb_build_object('event_id',event::text,'event_type','account.balance.changed.v2','account_id',source::text,
     'hold_id',h::text,'currency',currency_code,'available_minor',balance.available_minor::text,
     'ledger_minor',balance.ledger_minor::text,'reserved_minor',(balance.ledger_minor::numeric-balance.available_minor::numeric)::text,
     'balance_version',balance.balance_version::text,'occurred_at',clock_timestamp()),clock_timestamp());
 END IF; RETURN balance.balance_version;
END $$;

CREATE FUNCTION controlled_create_hold_v1(t uuid,a text,k text,c uuid,source uuid,destination uuid,amount bigint,currency_code text,ttl_seconds integer)
 RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE fp bytea;previous jsonb;new_hold_id uuid:=gen_random_uuid();at_time timestamptz;expires timestamptz;version bigint;source_account record; BEGIN
 IF source IS NULL OR destination IS NULL OR source=destination OR amount IS NULL OR amount<=0
  OR currency_code IS NULL OR currency_code!~'^[A-Z]{3}$' OR ttl_seconds IS NULL OR ttl_seconds NOT BETWEEN 1 AND 86400 THEN
  RAISE EXCEPTION 'invalid reservation' USING ERRCODE='22023';
 END IF;
 fp:=sha256(convert_to(jsonb_build_array(source,destination,amount::text,currency_code,ttl_seconds)::text,'UTF8'));
 previous:=public.hold_command_existing_v1(t,a,'create',k,c,fp); IF previous IS NOT NULL THEN RETURN previous; END IF;
 PERFORM 1 FROM public.tenant_transfer_policies WHERE tenant_id=t FOR UPDATE;
 PERFORM 1 FROM public.accounts WHERE tenant_id=t AND id IN (source,destination) ORDER BY id FOR UPDATE;
 PERFORM 1 FROM public.account_balance_projections WHERE account_id IN (source,destination) ORDER BY account_id FOR UPDATE;
 SELECT ac.status,ac.currency,ac.account_kind,b.available_minor INTO source_account FROM public.accounts ac
  JOIN public.account_balance_projections b ON b.account_id=ac.id WHERE ac.tenant_id=t AND ac.id=source;
 IF NOT FOUND OR source_account.account_kind<>'customer' OR source_account.currency<>currency_code
  OR NOT EXISTS(SELECT 1 FROM public.account_owners WHERE tenant_id=t AND account_id=source AND subject_id=a AND permission='debit')
  OR NOT EXISTS(SELECT 1 FROM public.account_credit_permissions WHERE tenant_id=t AND account_id=destination AND subject_id=a) THEN
  RAISE EXCEPTION 'reservation account denied' USING ERRCODE='42501';
 END IF;
 IF source_account.status<>'active' OR NOT EXISTS(SELECT 1 FROM public.accounts WHERE tenant_id=t AND id=destination AND status='active' AND currency=currency_code AND account_kind='customer') THEN
  RAISE EXCEPTION 'reservation account unavailable' USING ERRCODE='55000';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM public.tenant_transfer_policies WHERE tenant_id=t AND currency=currency_code) THEN
  RAISE EXCEPTION 'reservation currency policy denied' USING ERRCODE='42501';
 END IF;
 -- Keep this version bounded without deleting financial obligations.
 IF (SELECT count(*) FROM public.account_holds WHERE tenant_id=t AND state='active')>=1000 THEN
  RAISE EXCEPTION 'active reservation capacity reached' USING ERRCODE='54000';
 END IF;
 IF source_account.available_minor<amount THEN
  RAISE EXCEPTION 'insufficient spendable funds' USING ERRCODE='22023',CONSTRAINT='hold_insufficient_funds';
 END IF;
 at_time:=clock_timestamp();expires:=at_time+make_interval(secs=>ttl_seconds);
 INSERT INTO public.account_holds(id,tenant_id,debit_account_id,credit_account_id,amount_minor,currency,state,expires_at,created_at,created_by)
  VALUES(new_hold_id,t,source,destination,amount,currency_code,'active',expires,at_time,a);
 UPDATE public.account_balance_projections SET available_minor=available_minor-amount,balance_version=balance_version+1,updated_at=at_time WHERE account_id=source;
 INSERT INTO public.hold_expiry_jobs(hold_id,tenant_id,expires_at) VALUES(new_hold_id,t,expires);
 version:=public.hold_record_change_v1(t,a,new_hold_id,c,'active',true);
 RETURN public.hold_command_store_v1(t,a,'create',k,fp,jsonb_build_object('id',new_hold_id::text,'state','active','replayed',false,'source_balance_version',version::text,'expires_at',expires));
END $$;

CREATE FUNCTION controlled_decide_hold_v1(t uuid,a text,k text,c uuid,target uuid,decision text)
 RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE fp bytea;previous jsonb;hold public.account_holds%ROWTYPE;result jsonb;transfer_result jsonb;transfer_replayed boolean;version bigint;at_time timestamptz;failure_constraint text;failure_code text; BEGIN
 IF target IS NULL OR decision IS NULL OR decision NOT IN ('capture','release') THEN RAISE EXCEPTION 'invalid hold decision' USING ERRCODE='22023'; END IF;
 fp:=sha256(convert_to(jsonb_build_array(target,decision)::text,'UTF8'));
 previous:=public.hold_command_existing_v1(t,a,decision,k,c,fp); IF previous IS NOT NULL THEN RETURN previous; END IF;
 SELECT * INTO hold FROM public.account_holds WHERE id=target AND tenant_id=t;
 IF NOT FOUND THEN RAISE EXCEPTION 'hold not found' USING ERRCODE='P0002'; END IF;
 PERFORM 1 FROM public.tenant_transfer_policies WHERE tenant_id=t FOR UPDATE;
 PERFORM 1 FROM public.accounts WHERE tenant_id=t AND id IN (hold.debit_account_id,hold.credit_account_id) ORDER BY id FOR UPDATE;
 PERFORM 1 FROM public.account_balance_projections WHERE account_id IN (hold.debit_account_id,hold.credit_account_id) ORDER BY account_id FOR UPDATE;
 SELECT * INTO hold FROM public.account_holds WHERE id=target AND tenant_id=t FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM public.account_owners WHERE tenant_id=t AND account_id=hold.debit_account_id AND subject_id=a AND permission='debit') THEN
  RAISE EXCEPTION 'hold decision denied' USING ERRCODE='42501'; END IF;
 IF hold.state<>'active' THEN RAISE EXCEPTION 'hold already terminal' USING ERRCODE='23505',CONSTRAINT='hold_terminal_conflict'; END IF;
 at_time:=clock_timestamp();
 IF decision='release' THEN
  UPDATE public.account_balance_projections SET available_minor=available_minor+hold.amount_minor,balance_version=balance_version+1,updated_at=at_time WHERE account_id=hold.debit_account_id;
  UPDATE public.account_holds SET state='released',version=2,completed_at=at_time WHERE id=target;
  version:=public.hold_record_change_v1(t,a,target,c,'released',true);
  result:=jsonb_build_object('id',target::text,'state','released','replayed',false,'source_balance_version',version::text);
 ELSE
  IF at_time>=hold.expires_at THEN
   result:=jsonb_build_object('id',target::text,'state','active','replayed',false,'rejection_code','expired_capture');
  ELSE
   BEGIN
    -- This release is private to this transaction and does not advance a version.
    UPDATE public.account_balance_projections SET available_minor=available_minor+hold.amount_minor WHERE account_id=hold.debit_account_id;
    SELECT response_body,replayed INTO transfer_result,transfer_replayed FROM public.controlled_submit_transfer_v1(
     t,a,hold.debit_account_id,hold.credit_account_id,hold.amount_minor,hold.currency,
     'hold-capture-'||target::text,
     digest(concat_ws(E'\n',t::text,a,'transfers.create.v1',hold.debit_account_id::text,hold.credit_account_id::text,hold.currency,hold.amount_minor::text),'sha256'),
     c,hold.currency,at_time);
    IF transfer_result->>'status'<>'posted' OR transfer_replayed THEN
     RAISE EXCEPTION 'capture transfer denied' USING ERRCODE='P1601';
    END IF;
    UPDATE public.account_holds SET state='captured',version=2,completed_at=at_time,transfer_id=(transfer_result->>'transfer_id')::uuid WHERE id=target;
    version:=public.hold_record_change_v1(t,a,target,c,'captured',false);
    result:=jsonb_build_object('id',target::text,'state','captured','replayed',false,'source_balance_version',version::text,'transfer_id',transfer_result->>'transfer_id');
   EXCEPTION
    WHEN OTHERS THEN
     GET STACKED DIAGNOSTICS failure_constraint=CONSTRAINT_NAME,failure_code=RETURNED_SQLSTATE;
     IF failure_code<>'P1601' AND failure_constraint NOT IN (
      'controlled_transfer_policy_missing','controlled_transfer_currency','controlled_transfer_amount_minimum',
      'controlled_transfer_amount_maximum','controlled_transfer_account_status','controlled_transfer_account_currency',
      'controlled_transfer_source_actor','controlled_transfer_destination_actor','controlled_transfer_actor_velocity',
      'controlled_transfer_source_velocity','controlled_transfer_tenant_velocity') THEN RAISE; END IF;
     -- A business denial retains the complete original reservation. Do not catch
     -- serialization, cancellation, resource, invariant or infrastructure errors.
     result:=jsonb_build_object('id',target::text,'state','active','replayed',false,'rejection_code','capture_policy_denied');
   END;
  END IF;
  IF result ? 'rejection_code' THEN
   PERFORM public.controlled_append_audit_v1(gen_random_uuid(),t,a,'hold.capture_denied','hold',target::text,'failed',c,jsonb_build_object('reason',result->>'rejection_code'),clock_timestamp());
  END IF;
 END IF;
 IF result->>'state'<>'active' THEN UPDATE public.hold_expiry_jobs SET done=true WHERE hold_id=target; END IF;
 RETURN public.hold_command_store_v1(t,a,decision,k,fp,result);
END $$;

CREATE FUNCTION controlled_claim_hold_expiry_v1(batch integer,owner_id text,lease_seconds integer)
 RETURNS TABLE(hold_id uuid,tenant_id uuid,generation bigint) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF batch IS NULL OR batch NOT BETWEEN 1 AND 100 OR owner_id IS NULL OR length(btrim(owner_id)) NOT BETWEEN 1 AND 256 OR lease_seconds IS NULL OR lease_seconds NOT BETWEEN 1 AND 600 THEN
  RAISE EXCEPTION 'invalid expiry claim' USING ERRCODE='22023'; END IF;
 RETURN QUERY WITH candidates AS (
  SELECT j.hold_id FROM public.hold_expiry_jobs j WHERE NOT j.done AND j.expires_at<=clock_timestamp()
   AND (j.claimed_until IS NULL OR j.claimed_until<=clock_timestamp()) ORDER BY j.expires_at,j.hold_id LIMIT batch FOR UPDATE SKIP LOCKED
 ) UPDATE public.hold_expiry_jobs j SET claim_owner=owner_id,claim_generation=j.claim_generation+1,claimed_until=clock_timestamp()+make_interval(secs=>lease_seconds)
 FROM candidates c WHERE j.hold_id=c.hold_id RETURNING j.hold_id,j.tenant_id,j.claim_generation;
END $$;
CREATE FUNCTION controlled_expire_hold_v1(t uuid,target uuid,owner_id text,generation bigint)
 RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE hold public.account_holds%ROWTYPE;job public.hold_expiry_jobs%ROWTYPE; BEGIN
 IF public.tenant_scope_allows(t) IS DISTINCT FROM true OR current_setting('app.actor_id',true) IS DISTINCT FROM 'system:hold-expiry' THEN
  RAISE EXCEPTION 'expiry scope denied' USING ERRCODE='42501'; END IF;
 SELECT * INTO hold FROM public.account_holds WHERE id=target AND tenant_id=t;
 IF NOT FOUND THEN RAISE EXCEPTION 'hold not found' USING ERRCODE='P0002'; END IF;
 PERFORM 1 FROM public.accounts WHERE tenant_id=t AND id IN (hold.debit_account_id,hold.credit_account_id) ORDER BY id FOR UPDATE;
 PERFORM 1 FROM public.account_balance_projections WHERE account_id IN (hold.debit_account_id,hold.credit_account_id) ORDER BY account_id FOR UPDATE;
 SELECT * INTO hold FROM public.account_holds WHERE id=target AND tenant_id=t FOR UPDATE;
 SELECT * INTO job FROM public.hold_expiry_jobs WHERE hold_id=target AND tenant_id=t FOR UPDATE;
 IF NOT FOUND OR job.claim_owner IS DISTINCT FROM owner_id OR job.claim_generation IS DISTINCT FROM generation OR job.claimed_until<=clock_timestamp() OR job.claimed_until IS NULL THEN
  RAISE EXCEPTION 'stale expiry claim' USING ERRCODE='55000'; END IF;
 IF job.done OR hold.state<>'active' THEN RETURN false; END IF;
 IF hold.expires_at>clock_timestamp() THEN RAISE EXCEPTION 'hold is not due' USING ERRCODE='55000'; END IF;
 UPDATE public.account_balance_projections SET available_minor=available_minor+hold.amount_minor,balance_version=balance_version+1,updated_at=clock_timestamp() WHERE account_id=hold.debit_account_id;
 UPDATE public.account_holds SET state='expired',version=2,completed_at=clock_timestamp() WHERE id=target;
 PERFORM public.hold_record_change_v1(t,'system:hold-expiry',target,gen_random_uuid(),'expired',true);
 UPDATE public.hold_expiry_jobs SET done=true WHERE hold_id=target;
 RETURN true;
END $$;

REVOKE ALL ON account_holds,hold_commands,hold_events,hold_expiry_jobs FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE ON account_holds,hold_expiry_jobs TO ledgersync_migration_owner;
GRANT SELECT,INSERT ON hold_commands,hold_events TO ledgersync_migration_owner;
GRANT SELECT,UPDATE ON hold_expiry_jobs TO ledgersync_queue_owner;
DO $$ DECLARE signature text; BEGIN
 FOREACH signature IN ARRAY ARRAY[
  'hold_command_existing_v1(uuid,text,text,text,uuid,bytea)',
  'hold_command_store_v1(uuid,text,text,text,bytea,jsonb)',
  'hold_record_change_v1(uuid,text,uuid,uuid,text,boolean)',
  'controlled_create_hold_v1(uuid,text,text,uuid,uuid,uuid,bigint,text,integer)',
  'controlled_decide_hold_v1(uuid,text,text,uuid,uuid,text)',
  'controlled_expire_hold_v1(uuid,uuid,text,bigint)'] LOOP
  EXECUTE format('ALTER FUNCTION public.%s OWNER TO ledgersync_migration_owner',signature);
  EXECUTE format('REVOKE ALL ON FUNCTION public.%s FROM PUBLIC',signature);
 END LOOP;
END $$;
ALTER FUNCTION controlled_claim_hold_expiry_v1(integer,text,integer) OWNER TO ledgersync_queue_owner;
REVOKE ALL ON FUNCTION controlled_claim_hold_expiry_v1(integer,text,integer) FROM PUBLIC;
-- Existing installations can receive capabilities here. Fresh installations
-- create workload roles in the subsequent atomic role-policy step.
DO $workloads$ DECLARE workload text; BEGIN
 IF to_regrole('ledgersync_api') IS NOT NULL THEN
  GRANT SELECT ON account_holds,hold_events TO ledgersync_api;
  GRANT EXECUTE ON FUNCTION controlled_create_hold_v1(uuid,text,text,uuid,uuid,uuid,bigint,text,integer),controlled_decide_hold_v1(uuid,text,text,uuid,uuid,text) TO ledgersync_api;
 END IF;
 IF to_regrole('ledgersync_worker') IS NOT NULL THEN
  GRANT EXECUTE ON FUNCTION controlled_claim_hold_expiry_v1(integer,text,integer),controlled_expire_hold_v1(uuid,uuid,text,bigint) TO ledgersync_worker;
 END IF;
 FOREACH workload IN ARRAY ARRAY['ledgersync_api','ledgersync_worker','ledgersync_reconciliation','ledgersync_scheduler'] LOOP
  IF to_regrole(workload) IS NOT NULL THEN
   EXECUTE format('REVOKE INSERT,UPDATE,DELETE ON public.account_holds,public.hold_commands,public.hold_events,public.hold_expiry_jobs FROM %I',workload);
   IF workload IN ('ledgersync_reconciliation','ledgersync_scheduler') THEN
    EXECUTE format('GRANT SELECT ON public.account_holds TO %I',workload);
   END IF;
  END IF;
 END LOOP;
END $workloads$;

-- Correct the receipt meaning for new postings without changing saved receipts.
-- Retain the actor guard and all authorization/policy/journal logic unchanged.
DO $receipt$
DECLARE routine record; marker text:=$m$'posted_minor',v_source_available::text,'version'$m$; replacement text:=$m$'posted_minor',v_source_ledger::text,'available_minor',v_source_available::text,'reserved_minor',(v_source_ledger::numeric-v_source_available::numeric)::text,'version'$m$;
BEGIN
 SELECT p.oid,p.prosrc INTO STRICT routine FROM pg_proc p WHERE p.oid='public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid,text,timestamptz)'::regprocedure;
 IF (length(routine.prosrc)-length(replace(routine.prosrc,marker,'')))/length(marker)<>1 THEN RAISE EXCEPTION 'unexpected transfer receipt shape'; END IF;
 EXECUTE replace(pg_get_functiondef(routine.oid),routine.prosrc,replace(routine.prosrc,marker,replacement));
END $receipt$;

REVOKE ALL ON FUNCTION protect_hold_history_v1(),enrich_reserved_balance_event_v1() FROM PUBLIC;
ALTER FUNCTION protect_hold_history_v1() OWNER TO ledgersync_migration_owner;
ALTER FUNCTION enrich_reserved_balance_event_v1() OWNER TO ledgersync_migration_owner;

-- Account closure also protects incoming reservations on a zero-balance target.
DO $closure$
DECLARE routine record; marker text:=$m$IF p_status='closed' THEN$m$; replacement text:=$m$IF p_status='closed' THEN
   IF EXISTS(SELECT 1 FROM public.account_holds WHERE tenant_id=p_tenant AND state='active' AND (debit_account_id=p_account OR credit_account_id=p_account)) THEN
    RAISE EXCEPTION 'account has active reservation obligations' USING ERRCODE='23514';
   END IF;$m$;
BEGIN
 SELECT p.oid,p.prosrc INTO STRICT routine FROM pg_proc p WHERE p.oid='public.controlled_update_account_v1(uuid,text,uuid,bigint,text,text,text,text,uuid,timestamptz,text)'::regprocedure;
 IF (length(routine.prosrc)-length(replace(routine.prosrc,marker,'')))/length(marker)<>1 THEN RAISE EXCEPTION 'unexpected account closure shape'; END IF;
 EXECUTE replace(pg_get_functiondef(routine.oid),routine.prosrc,replace(routine.prosrc,marker,replacement));
END $closure$;
