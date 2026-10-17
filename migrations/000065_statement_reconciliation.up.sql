-- Synthetic CSV statement comparison. The owner has no financial write grants.
-- Existing journals, postings, transfers and balances are never altered here.
DO $$ BEGIN
 IF to_regrole('ledgersync_statement_owner') IS NULL THEN
  CREATE ROLE ledgersync_statement_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
 END IF;
 IF EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname='ledgersync_statement_owner' AND
  (r.rolcanlogin OR r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication OR r.rolbypassrls
   OR EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid OR m.roleid=r.oid))) THEN
  RAISE EXCEPTION 'unsafe statement function owner' USING ERRCODE='42501';
 END IF;
END $$;
CREATE TABLE statement_mappings(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL REFERENCES tenants(id),source_account_ref text NOT NULL CHECK(source_account_ref~'^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$'),
 account_id uuid NOT NULL,currency text NOT NULL CHECK(currency='INR'),version integer NOT NULL DEFAULT 1 CHECK(version=1),
 created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(tenant_id),UNIQUE(id,tenant_id),FOREIGN KEY(account_id,tenant_id) REFERENCES accounts(id,tenant_id));
CREATE TABLE statement_imports(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL,mapping_id uuid NOT NULL,source_sha256 bytea NOT NULL CHECK(octet_length(source_sha256)=32),
 source_bytes bytea NOT NULL CHECK(octet_length(source_bytes) BETWEEN 1 AND 1048576),
 period_start timestamptz NOT NULL,period_end timestamptz NOT NULL CHECK(period_end>period_start),
 opening_minor bigint NOT NULL,closing_minor bigint NOT NULL,row_count integer NOT NULL CHECK(row_count BETWEEN 1 AND 1000),
 created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK(sha256(source_bytes)=source_sha256),UNIQUE(mapping_id,source_sha256),UNIQUE(id,tenant_id,mapping_id),
 FOREIGN KEY(mapping_id,tenant_id) REFERENCES statement_mappings(id,tenant_id));
CREATE TABLE statement_rows(
 import_id uuid NOT NULL,tenant_id uuid NOT NULL,mapping_id uuid NOT NULL,row_id text NOT NULL CHECK(row_id~'^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$'),
 external_reference text NOT NULL CHECK(external_reference~'^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$'),direction text NOT NULL CHECK(direction IN ('debit','credit')),
 amount_minor bigint NOT NULL CHECK(amount_minor>0),currency text NOT NULL CHECK(currency='INR'),posted_at timestamptz NOT NULL,
 PRIMARY KEY(import_id,row_id),UNIQUE(mapping_id,row_id),FOREIGN KEY(import_id,tenant_id,mapping_id) REFERENCES statement_imports(id,tenant_id,mapping_id));
CREATE TABLE statement_runs(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL,import_id uuid NOT NULL,mapping_id uuid NOT NULL,rule_version integer NOT NULL DEFAULT 1 CHECK(rule_version=1),
 cutoff timestamptz NOT NULL,correlation_id uuid NOT NULL,state text NOT NULL CHECK(state IN ('prepared','completed')),result jsonb,
 created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),completed_at timestamptz,
 CHECK((state='prepared' AND result IS NULL AND completed_at IS NULL) OR (state='completed' AND jsonb_typeof(result)='object' AND completed_at IS NOT NULL)),
 UNIQUE(import_id,rule_version),UNIQUE(id,tenant_id,mapping_id),FOREIGN KEY(import_id,tenant_id,mapping_id) REFERENCES statement_imports(id,tenant_id,mapping_id));
CREATE TABLE statement_snapshot_items(
 run_id uuid NOT NULL,tenant_id uuid NOT NULL,mapping_id uuid NOT NULL,posting_id uuid NOT NULL REFERENCES ledger_postings(id),
 external_reference text NOT NULL,direction text NOT NULL CHECK(direction IN ('debit','credit')),amount_minor bigint NOT NULL CHECK(amount_minor>0),currency text NOT NULL CHECK(currency='INR'),
 occurred_at timestamptz NOT NULL,recorded_at timestamptz NOT NULL,PRIMARY KEY(run_id,posting_id),
 FOREIGN KEY(run_id,tenant_id,mapping_id) REFERENCES statement_runs(id,tenant_id,mapping_id));
CREATE TABLE statement_matches(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL,mapping_id uuid NOT NULL,run_id uuid NOT NULL,row_id text NOT NULL,posting_id uuid NOT NULL,
 decision text NOT NULL CHECK(decision IN ('exact','manual')),created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(mapping_id,row_id),UNIQUE(mapping_id,posting_id),FOREIGN KEY(run_id,tenant_id,mapping_id) REFERENCES statement_runs(id,tenant_id,mapping_id),
 FOREIGN KEY(mapping_id,row_id) REFERENCES statement_rows(mapping_id,row_id),FOREIGN KEY(run_id,posting_id) REFERENCES statement_snapshot_items(run_id,posting_id));
CREATE TABLE statement_review_items(
 tenant_id uuid NOT NULL,run_id uuid NOT NULL,mapping_id uuid NOT NULL,kind text NOT NULL CHECK(kind IN ('statement','ledger')),item_id text NOT NULL,
 version bigint NOT NULL DEFAULT 0 CHECK(version>=0),classification text NOT NULL,
 status text NOT NULL CHECK(status IN ('open','reviewed_exception','matched')),assignee text NOT NULL DEFAULT '',reason text NOT NULL DEFAULT '',
 comment text NOT NULL DEFAULT '' CHECK(octet_length(comment)<=512),evidence text NOT NULL DEFAULT '' CHECK(octet_length(evidence)<=256),
 updated_by text NOT NULL,updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(run_id,kind,item_id),
 FOREIGN KEY(run_id,tenant_id,mapping_id) REFERENCES statement_runs(id,tenant_id,mapping_id));
CREATE TABLE statement_review_events(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL,run_id uuid NOT NULL,kind text NOT NULL,item_id text NOT NULL,
 from_version bigint NOT NULL,to_version bigint NOT NULL CHECK(to_version=from_version+1),decision jsonb NOT NULL,
 actor_subject_id text NOT NULL,correlation_id uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(run_id,kind,item_id) REFERENCES statement_review_items(run_id,kind,item_id));
CREATE TABLE statement_commands(
 tenant_id uuid NOT NULL REFERENCES tenants(id),actor_subject_id text NOT NULL,operation text NOT NULL,idempotency_key text NOT NULL,
 fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32),response jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,actor_subject_id,operation,idempotency_key));
CREATE INDEX statement_imports_recent_idx ON statement_imports(tenant_id,created_at DESC,id DESC);
CREATE INDEX statement_review_queue_idx ON statement_review_items(tenant_id,run_id,kind,item_id);
DO $$ DECLARE relation text;BEGIN
 FOREACH relation IN ARRAY ARRAY['statement_mappings','statement_imports','statement_rows','statement_runs','statement_snapshot_items','statement_matches','statement_review_items','statement_review_events','statement_commands'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',relation);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',relation);
  EXECUTE format('CREATE POLICY tenant_boundary ON %I USING (public.tenant_scope_allows(tenant_id)) WITH CHECK (public.tenant_scope_allows(tenant_id))',relation);
 END LOOP;
 FOREACH relation IN ARRAY ARRAY['statement_mappings','statement_imports','statement_rows','statement_snapshot_items','statement_matches','statement_review_events','statement_commands'] LOOP
  EXECUTE format('CREATE TRIGGER statement_append_only BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION reject_row_mutation()',relation);
 END LOOP;
END $$;
CREATE FUNCTION protect_statement_run_v1() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF TG_OP='DELETE' OR OLD.state<>'prepared' OR NEW.state<>'completed'
  OR (to_jsonb(NEW)-ARRAY['state','result','completed_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['state','result','completed_at']) THEN
  RAISE EXCEPTION 'statement run history is immutable' USING ERRCODE='55000';END IF;RETURN NEW;END $$;
CREATE TRIGGER statement_run_history BEFORE UPDATE OR DELETE ON statement_runs FOR EACH ROW EXECUTE FUNCTION protect_statement_run_v1();
CREATE FUNCTION protect_statement_review_v1() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF TG_OP='DELETE' OR NEW.version<>OLD.version+1 OR
  (to_jsonb(NEW)-ARRAY['version','status','assignee','reason','comment','evidence','updated_by','updated_at']) IS DISTINCT FROM
  (to_jsonb(OLD)-ARRAY['version','status','assignee','reason','comment','evidence','updated_by','updated_at']) THEN
  RAISE EXCEPTION 'statement review identity or version is immutable' USING ERRCODE='55000';END IF;RETURN NEW;END $$;
CREATE TRIGGER statement_review_history BEFORE UPDATE OR DELETE ON statement_review_items FOR EACH ROW EXECUTE FUNCTION protect_statement_review_v1();

-- Private invoker helper. It is not callable by workload roles.
CREATE FUNCTION statement_existing_command_v1(t uuid,a text,op text,k text,c uuid,fp bytea) RETURNS jsonb
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ DECLARE previous record;BEGIN
 IF public.tenant_scope_allows(t) IS DISTINCT FROM true OR a IS DISTINCT FROM current_setting('app.actor_id',true) THEN
  RAISE EXCEPTION 'statement scope denied' USING ERRCODE='42501';END IF;
 IF c IS NULL OR op NOT IN ('mapping','import','prepare','review') OR k IS NULL OR octet_length(k) NOT BETWEEN 16 AND 255 OR k!~'^[!-~]+$' OR octet_length(fp)<>32 THEN
  RAISE EXCEPTION 'invalid statement command' USING ERRCODE='22023';END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('statement-command|'||t::text||'|'||a||'|'||op||'|'||k,0));
 SELECT fingerprint,response INTO previous FROM public.statement_commands WHERE tenant_id=t AND actor_subject_id=a AND operation=op AND idempotency_key=k;
 IF FOUND THEN
  IF previous.fingerprint<>fp THEN RAISE EXCEPTION 'statement intent conflict' USING ERRCODE='23505',CONSTRAINT='statement_intent_conflict';END IF;
  RETURN previous.response||jsonb_build_object('replayed',true);
 END IF;RETURN NULL;END $$;
CREATE FUNCTION statement_store_command_v1(t uuid,a text,op text,k text,c uuid,fp bytea,resource uuid) RETURNS jsonb
 LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ DECLARE result jsonb:=jsonb_build_object('id',resource::text,'replayed',false);BEGIN
 INSERT INTO public.statement_commands(tenant_id,actor_subject_id,operation,idempotency_key,fingerprint,response) VALUES(t,a,op,k,fp,result);
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),t,a,'statement.'||op,'statement',resource::text,'succeeded',c,jsonb_build_object('operation',op),clock_timestamp());
 RETURN result;END $$;

CREATE FUNCTION controlled_statement_mapping_v1(t uuid,a text,k text,c uuid,account uuid,source_ref text) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE fp bytea:=sha256(convert_to(jsonb_build_array(account,source_ref)::text,'UTF8'));previous jsonb;mapping public.statement_mappings%ROWTYPE;BEGIN
 previous:=public.statement_existing_command_v1(t,a,'mapping',k,c,fp);IF previous IS NOT NULL THEN RETURN previous;END IF;
 IF source_ref IS NULL OR source_ref!~'^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$' OR NOT EXISTS(SELECT 1 FROM public.accounts WHERE id=account AND tenant_id=t AND currency='INR' AND status='active') THEN
  RAISE EXCEPTION 'invalid statement mapping' USING ERRCODE='22023';END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('statement-mapping|'||t::text,0));
 SELECT * INTO mapping FROM public.statement_mappings WHERE tenant_id=t;
 IF FOUND THEN IF mapping.account_id<>account OR mapping.source_account_ref<>source_ref THEN RAISE EXCEPTION 'mapping is already fixed' USING ERRCODE='23505';END IF;
 ELSE INSERT INTO public.statement_mappings(id,tenant_id,source_account_ref,account_id,currency,created_by) VALUES(gen_random_uuid(),t,source_ref,account,'INR',a) RETURNING * INTO mapping;END IF;
 RETURN public.statement_store_command_v1(t,a,'mapping',k,c,fp,mapping.id);END $$;

CREATE FUNCTION controlled_statement_import_v1(t uuid,a text,k text,c uuid,mapping uuid,source bytea) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE fp bytea;previous jsonb;binding public.statement_mappings%ROWTYPE;import_id uuid;lines text[];fields text[];row_data jsonb:='[]';
 source_text text;line text;metadata text[];current_metadata text[];row_ids text[]:=ARRAY[]::text[];n integer:=0;amount bigint;opening bigint;closing bigint;
 first_at timestamptz;last_at timestamptz;posted timestamptz;total numeric:=0;digest bytea;BEGIN
 IF source IS NULL OR octet_length(source) NOT BETWEEN 1 AND 1048576 THEN RAISE EXCEPTION 'statement source size' USING ERRCODE='22023';END IF;
 digest:=sha256(source);fp:=sha256(convert_to(jsonb_build_array(mapping,encode(digest,'hex'))::text,'UTF8'));
 previous:=public.statement_existing_command_v1(t,a,'import',k,c,fp);IF previous IS NOT NULL THEN RETURN previous;END IF;
 SELECT * INTO binding FROM public.statement_mappings WHERE id=mapping AND tenant_id=t FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement mapping not found' USING ERRCODE='P0002';END IF;
 SELECT id INTO import_id FROM public.statement_imports WHERE mapping_id=mapping AND source_sha256=digest;
 IF FOUND THEN RETURN public.statement_store_command_v1(t,a,'import',k,c,fp,import_id)||jsonb_build_object('replayed',true);END IF;
 IF (SELECT count(*) FROM public.statement_imports WHERE tenant_id=t)>=100 THEN RAISE EXCEPTION 'statement import quota' USING ERRCODE='54000';END IF;
 source_text:=replace(convert_from(source,'UTF8'),E'\r\n',E'\n');
 IF source_text LIKE '%"%' OR position(E'\r' IN source_text)>0 THEN RAISE EXCEPTION 'quoted or invalid CSV' USING ERRCODE='22023';END IF;
 lines:=string_to_array(source_text,E'\n');IF lines[array_length(lines,1)]='' THEN lines:=lines[1:array_length(lines,1)-1];END IF;
 IF array_length(lines,1) NOT BETWEEN 2 AND 1001 OR lines[1]<>'row_id,source_account_ref,period_start,period_end,posted_at,external_reference,direction,amount_minor,currency,opening_minor,closing_minor' THEN RAISE EXCEPTION 'statement CSV header or rows' USING ERRCODE='22023';END IF;
 FOREACH line IN ARRAY lines[2:array_length(lines,1)] LOOP
  fields:=string_to_array(line,',');n:=n+1;
  IF cardinality(fields)<>11 OR fields[1]!~'^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$' OR fields[2]<>binding.source_account_ref
   OR fields[6]!~'^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$' OR fields[1]=ANY(row_ids) OR fields[7] NOT IN ('debit','credit') OR fields[9]<>'INR'
   OR fields[8]!~'^[1-9][0-9]{0,18}$' OR fields[10]!~'^(0|-?[1-9][0-9]{0,18})$' OR fields[11]!~'^(0|-?[1-9][0-9]{0,18})$'
   OR fields[3]!~'^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' OR fields[4]!~'^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$'
   OR fields[5]!~'^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' THEN RAISE EXCEPTION 'invalid statement CSV row' USING ERRCODE='22023';END IF;
  amount:=fields[8]::bigint;opening:=fields[10]::bigint;closing:=fields[11]::bigint;first_at:=fields[3]::timestamptz;last_at:=fields[4]::timestamptz;posted:=fields[5]::timestamptz;
  IF last_at<=first_at OR posted<first_at OR posted>=last_at OR to_char(first_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')<>fields[3]
   OR to_char(last_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')<>fields[4] OR to_char(posted AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')<>fields[5] THEN RAISE EXCEPTION 'invalid statement period' USING ERRCODE='22023';END IF;
  current_metadata:=ARRAY[fields[2],fields[3],fields[4],fields[9],fields[10],fields[11]];
  IF metadata IS NULL THEN metadata:=current_metadata;ELSIF metadata<>current_metadata THEN RAISE EXCEPTION 'mixed statement metadata' USING ERRCODE='22023';END IF;
  row_ids:=array_append(row_ids,fields[1]);total:=total+CASE WHEN fields[7]='debit' THEN -amount::numeric ELSE amount::numeric END;
  row_data:=row_data||jsonb_build_object('row_id',fields[1],'reference',fields[6],'direction',fields[7],'minor',amount::text,'posted_at',posted);
 END LOOP;
 IF opening::numeric+total<>closing::numeric THEN RAISE EXCEPTION 'statement total mismatch' USING ERRCODE='22023';END IF;
 IF EXISTS(SELECT 1 FROM public.statement_rows WHERE mapping_id=mapping AND row_id=ANY(row_ids)) THEN RAISE EXCEPTION 'statement source row reused' USING ERRCODE='23505',CONSTRAINT='statement_source_row_reused';END IF;
 import_id:=gen_random_uuid();INSERT INTO public.statement_imports(id,tenant_id,mapping_id,source_sha256,source_bytes,period_start,period_end,opening_minor,closing_minor,row_count,created_by)
 VALUES(import_id,t,mapping,digest,source,first_at,last_at,opening,closing,n,a);
 INSERT INTO public.statement_rows(import_id,tenant_id,mapping_id,row_id,external_reference,direction,amount_minor,currency,posted_at)
 SELECT import_id,t,mapping,item->>'row_id',item->>'reference',item->>'direction',(item->>'minor')::bigint,'INR',(item->>'posted_at')::timestamptz FROM jsonb_array_elements(row_data) item;
 RETURN public.statement_store_command_v1(t,a,'import',k,c,fp,import_id);END $$;

CREATE FUNCTION controlled_statement_prepare_v1(t uuid,a text,k text,c uuid,source_import uuid) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE fp bytea:=sha256(convert_to(jsonb_build_array(source_import,1)::text,'UTF8'));previous jsonb;source public.statement_imports%ROWTYPE;
 binding public.statement_mappings%ROWTYPE;run_id uuid;at_time timestamptz:=clock_timestamp();item_count integer;BEGIN
 previous:=public.statement_existing_command_v1(t,a,'prepare',k,c,fp);IF previous IS NOT NULL THEN RETURN previous;END IF;
 SELECT * INTO source FROM public.statement_imports WHERE id=source_import AND tenant_id=t;IF NOT FOUND THEN RAISE EXCEPTION 'statement import not found' USING ERRCODE='P0002';END IF;
 SELECT * INTO binding FROM public.statement_mappings WHERE id=source.mapping_id AND tenant_id=t FOR UPDATE;
 SELECT id INTO run_id FROM public.statement_runs WHERE import_id=source_import AND rule_version=1;
 IF FOUND THEN RETURN public.statement_store_command_v1(t,a,'prepare',k,c,fp,run_id)||jsonb_build_object('replayed',true);END IF;
 run_id:=gen_random_uuid();INSERT INTO public.statement_runs(id,tenant_id,import_id,mapping_id,cutoff,correlation_id,state,created_by) VALUES(run_id,t,source_import,binding.id,at_time,c,'prepared',a);
 INSERT INTO public.statement_snapshot_items(run_id,tenant_id,mapping_id,posting_id,external_reference,direction,amount_minor,currency,occurred_at,recorded_at)
 SELECT run_id,t,binding.id,p.id,COALESCE(j.transfer_id::text,j.funding_event_id::text),p.direction,p.amount_minor,p.currency,p.occurred_at,p.created_at
 FROM public.ledger_postings p JOIN public.journal_transactions j ON j.id=p.journal_transaction_id
 WHERE j.tenant_id=t AND p.account_id=binding.account_id AND p.currency='INR' AND p.occurred_at>=source.period_start AND p.occurred_at<source.period_end
  AND p.created_at<=at_time AND NOT EXISTS(SELECT 1 FROM public.statement_matches used WHERE used.mapping_id=binding.id AND used.posting_id=p.id)
 ORDER BY p.id LIMIT 2001;
 GET DIAGNOSTICS item_count=ROW_COUNT;IF item_count>2000 THEN RAISE EXCEPTION 'statement snapshot limit' USING ERRCODE='54000';END IF;
 RETURN public.statement_store_command_v1(t,a,'prepare',k,c,fp,run_id);END $$;

CREATE FUNCTION controlled_statement_complete_v1(t uuid,a text,target_run uuid,proposed jsonb) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE run public.statement_runs%ROWTYPE;r record;candidate_ids text[];candidate_count integer;source_count integer;expected text;observed jsonb;matching jsonb;
 row_results jsonb:='[]';match_results jsonb:='[]';unmatched jsonb;chosen uuid;matched_count integer:=0;BEGIN
 IF public.tenant_scope_allows(t) IS DISTINCT FROM true OR a IS DISTINCT FROM current_setting('app.actor_id',true) THEN RAISE EXCEPTION 'statement scope denied' USING ERRCODE='42501';END IF;
 SELECT * INTO run FROM public.statement_runs WHERE id=target_run AND tenant_id=t;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement run not found' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM public.statement_mappings WHERE id=run.mapping_id AND tenant_id=t FOR UPDATE;
 SELECT * INTO run FROM public.statement_runs WHERE id=target_run AND tenant_id=t FOR UPDATE;
 IF run.created_by<>a THEN RAISE EXCEPTION 'statement run owner differs' USING ERRCODE='42501';END IF;
 IF run.state='completed' THEN RETURN jsonb_build_object('id',target_run::text,'replayed',true);END IF;
 IF jsonb_typeof(proposed->'rows') IS DISTINCT FROM 'array' OR jsonb_typeof(proposed->'matches') IS DISTINCT FROM 'array'
  OR octet_length(proposed::text)>1048576 OR jsonb_array_length(proposed->'rows')<>(SELECT count(*) FROM public.statement_rows WHERE import_id=run.import_id)
  OR (SELECT count(DISTINCT item->>'row_id') FROM jsonb_array_elements(proposed->'rows') item)<>jsonb_array_length(proposed->'rows') THEN RAISE EXCEPTION 'invalid statement match result' USING ERRCODE='22023';END IF;
 FOR r IN SELECT * FROM public.statement_rows WHERE import_id=run.import_id ORDER BY row_id LOOP
  SELECT COALESCE(array_agg(posting_id::text ORDER BY posting_id),ARRAY[]::text[]) INTO candidate_ids FROM public.statement_snapshot_items
   WHERE run_id=target_run AND external_reference=r.external_reference AND direction=r.direction AND amount_minor=r.amount_minor AND currency=r.currency;
  candidate_count:=cardinality(candidate_ids);
  SELECT count(*) INTO source_count FROM public.statement_rows WHERE import_id=run.import_id AND external_reference=r.external_reference AND direction=r.direction AND amount_minor=r.amount_minor AND currency=r.currency;
  expected:=CASE WHEN candidate_count=1 AND source_count=1 THEN 'matched' WHEN candidate_count>0 THEN 'ambiguous'
    WHEN EXISTS(SELECT 1 FROM public.statement_snapshot_items WHERE run_id=target_run AND external_reference=r.external_reference) THEN 'amount_or_direction_difference' ELSE 'unmatched_external' END;
  SELECT item INTO observed FROM jsonb_array_elements(proposed->'rows') item WHERE item->>'row_id'=r.row_id;
  IF observed IS NULL OR observed->>'status' IS DISTINCT FROM expected OR observed->'candidates' IS DISTINCT FROM to_jsonb(candidate_ids) THEN RAISE EXCEPTION 'match differs from frozen snapshot' USING ERRCODE='22023';END IF;
  IF expected='matched' THEN
   matched_count:=matched_count+1;
   IF (SELECT count(*) FROM jsonb_array_elements(proposed->'matches') item WHERE item->>'row_id'=r.row_id AND item->>'posting_id'=candidate_ids[1])<>1 THEN RAISE EXCEPTION 'missing exact match' USING ERRCODE='22023';END IF;
   chosen:=candidate_ids[1]::uuid;
   IF EXISTS(SELECT 1 FROM public.statement_matches WHERE mapping_id=run.mapping_id AND posting_id=chosen) THEN expected:='already_consumed';
   ELSE INSERT INTO public.statement_matches(id,tenant_id,mapping_id,run_id,row_id,posting_id,decision,created_by) VALUES(gen_random_uuid(),t,run.mapping_id,target_run,r.row_id,chosen,'exact',a);
    match_results:=match_results||jsonb_build_object('row_id',r.row_id,'posting_id',chosen::text);
   END IF;
  ELSIF EXISTS(SELECT 1 FROM jsonb_array_elements(proposed->'matches') item WHERE item->>'row_id'=r.row_id) THEN RAISE EXCEPTION 'nonunique proposed match' USING ERRCODE='22023';END IF;
  row_results:=row_results||jsonb_build_object('row_id',r.row_id,'status',expected,'candidates',to_jsonb(candidate_ids));
  INSERT INTO public.statement_review_items(tenant_id,run_id,mapping_id,kind,item_id,classification,status,updated_by)
   VALUES(t,target_run,run.mapping_id,'statement',r.row_id,expected,CASE WHEN expected='matched' THEN 'matched' ELSE 'open' END,a);
 END LOOP;
 IF jsonb_array_length(proposed->'matches')<>matched_count THEN RAISE EXCEPTION 'extraneous proposed matches' USING ERRCODE='22023';END IF;
 SELECT COALESCE(jsonb_agg(s.posting_id::text ORDER BY s.posting_id),'[]'::jsonb) INTO unmatched FROM public.statement_snapshot_items s
 WHERE s.run_id=target_run AND NOT EXISTS(SELECT 1 FROM public.statement_matches m WHERE m.run_id=target_run AND m.posting_id=s.posting_id);
 INSERT INTO public.statement_review_items(tenant_id,run_id,mapping_id,kind,item_id,classification,status,updated_by)
 SELECT t,target_run,run.mapping_id,'ledger',s.posting_id::text,
  CASE WHEN EXISTS(SELECT 1 FROM public.statement_matches m WHERE m.mapping_id=run.mapping_id AND m.posting_id=s.posting_id) THEN 'already_consumed' ELSE 'unmatched_internal' END,'open',a
 FROM public.statement_snapshot_items s WHERE s.run_id=target_run AND NOT EXISTS(SELECT 1 FROM public.statement_matches m WHERE m.run_id=target_run AND m.posting_id=s.posting_id);
 UPDATE public.statement_runs SET state='completed',completed_at=clock_timestamp(),result=jsonb_build_object('rows',row_results,'matches',match_results,'unmatched_postings',unmatched) WHERE id=target_run;
 PERFORM public.controlled_append_audit_v1(gen_random_uuid(),t,a,'statement.completed','statement',target_run::text,'succeeded',run.correlation_id,jsonb_build_object('rule_version',1),clock_timestamp());
 RETURN jsonb_build_object('id',target_run::text,'replayed',false);END $$;

CREATE FUNCTION controlled_statement_review_v1(t uuid,a text,k text,c uuid,target_run uuid,decision jsonb) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
 DECLARE fp bytea:=sha256(convert_to(jsonb_build_array(target_run,decision)::text,'UTF8'));previous jsonb;run public.statement_runs%ROWTYPE;
 item public.statement_review_items%ROWTYPE;ledger_item public.statement_review_items%ROWTYPE;source public.statement_rows%ROWTYPE;snapshot public.statement_snapshot_items%ROWTYPE;
 expected_version bigint;target_posting uuid;existing_posting uuid;BEGIN
 previous:=public.statement_existing_command_v1(t,a,'review',k,c,fp);IF previous IS NOT NULL THEN RETURN previous;END IF;
 IF jsonb_typeof(decision) IS DISTINCT FROM 'object' OR octet_length(decision::text)>4096
  OR EXISTS(SELECT 1 FROM jsonb_object_keys(decision) key WHERE key NOT IN ('kind','item_id','expected_version','status','assignee','reason','comment','evidence','posting_id'))
  OR decision->>'kind' NOT IN ('statement','ledger') OR decision->>'status' NOT IN ('open','reviewed_exception','matched')
  OR jsonb_typeof(decision->'expected_version') IS DISTINCT FROM 'string' OR decision->>'expected_version'!~'^(0|[1-9][0-9]{0,18})$'
  OR jsonb_typeof(decision->'assignee') IS DISTINCT FROM 'string' OR octet_length(decision->>'assignee')>256 OR (decision->>'assignee')~'[[:cntrl:]]'
  OR jsonb_typeof(decision->'reason') IS DISTINCT FROM 'string' OR decision->>'reason'!~'^[a-z][a-z_]{0,63}$'
  OR jsonb_typeof(decision->'comment') IS DISTINCT FROM 'string' OR octet_length(decision->>'comment')>512
  OR jsonb_typeof(decision->'evidence') IS DISTINCT FROM 'string' OR octet_length(decision->>'evidence')>256
  OR (decision->>'evidence'<>'' AND decision->>'evidence'!~'^/(transfers|funding|reconciliation)/[A-Za-z0-9/_-]+$') THEN RAISE EXCEPTION 'invalid statement review' USING ERRCODE='22023';END IF;
 expected_version:=(decision->>'expected_version')::bigint;
 SELECT * INTO run FROM public.statement_runs WHERE id=target_run AND tenant_id=t;
 IF NOT FOUND OR run.state<>'completed' THEN RAISE EXCEPTION 'completed statement run not found' USING ERRCODE='P0002';END IF;
 PERFORM 1 FROM public.statement_mappings WHERE id=run.mapping_id AND tenant_id=t FOR UPDATE;
 SELECT * INTO item FROM public.statement_review_items WHERE run_id=target_run AND kind=decision->>'kind' AND item_id=decision->>'item_id' AND tenant_id=t FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement review item not found' USING ERRCODE='P0002';END IF;
 IF item.version<>expected_version OR item.version=9223372036854775807 THEN RAISE EXCEPTION 'stale statement review version' USING ERRCODE='23505',CONSTRAINT='statement_review_version';END IF;
 IF decision->>'assignee'<>'' AND NOT EXISTS(SELECT 1 FROM public.tenant_subject_roles WHERE tenant_id=t AND subject_id=decision->>'assignee') THEN RAISE EXCEPTION 'assignee is not in tenant' USING ERRCODE='22023';END IF;
 IF item.kind='statement' THEN SELECT posting_id INTO existing_posting FROM public.statement_matches WHERE run_id=target_run AND row_id=item.item_id;END IF;
 IF existing_posting IS NOT NULL AND (decision->>'status'<>'matched' OR NULLIF(decision->>'posting_id','')::uuid IS DISTINCT FROM existing_posting) THEN RAISE EXCEPTION 'a recorded match cannot be replaced' USING ERRCODE='23505';END IF;
 IF decision->>'status'='matched' THEN
  IF item.kind<>'statement' OR NULLIF(decision->>'posting_id','') IS NULL THEN RAISE EXCEPTION 'manual statement link required' USING ERRCODE='22023';END IF;
  target_posting:=(decision->>'posting_id')::uuid;
  SELECT * INTO source FROM public.statement_rows WHERE import_id=run.import_id AND row_id=item.item_id;
  SELECT * INTO snapshot FROM public.statement_snapshot_items WHERE run_id=target_run AND posting_id=target_posting;
  IF NOT FOUND OR source.amount_minor<>snapshot.amount_minor OR source.direction<>snapshot.direction OR source.currency<>snapshot.currency THEN RAISE EXCEPTION 'manual link differs in money or direction' USING ERRCODE='22023';END IF;
  IF existing_posting IS NULL THEN
   INSERT INTO public.statement_matches(id,tenant_id,mapping_id,run_id,row_id,posting_id,decision,created_by) VALUES(gen_random_uuid(),t,run.mapping_id,target_run,item.item_id,target_posting,'manual',a);
   SELECT * INTO ledger_item FROM public.statement_review_items WHERE run_id=target_run AND kind='ledger' AND item_id=target_posting::text FOR UPDATE;
   IF FOUND THEN
    UPDATE public.statement_review_items SET version=version+1,status='matched',reason='linked_statement',updated_by=a,updated_at=clock_timestamp() WHERE run_id=target_run AND kind='ledger' AND item_id=target_posting::text;
    INSERT INTO public.statement_review_events(id,tenant_id,run_id,kind,item_id,from_version,to_version,decision,actor_subject_id,correlation_id)
     VALUES(gen_random_uuid(),t,target_run,'ledger',target_posting::text,ledger_item.version,ledger_item.version+1,jsonb_build_object('status','matched','reason','linked_statement'),a,c);
   END IF;
  END IF;
 ELSIF NULLIF(decision->>'posting_id','') IS NOT NULL THEN RAISE EXCEPTION 'only matched decisions link a posting' USING ERRCODE='22023';END IF;
 UPDATE public.statement_review_items SET version=version+1,status=decision->>'status',assignee=decision->>'assignee',reason=decision->>'reason',comment=decision->>'comment',evidence=decision->>'evidence',updated_by=a,updated_at=clock_timestamp()
 WHERE run_id=target_run AND kind=item.kind AND item_id=item.item_id;
 INSERT INTO public.statement_review_events(id,tenant_id,run_id,kind,item_id,from_version,to_version,decision,actor_subject_id,correlation_id)
 VALUES(gen_random_uuid(),t,target_run,item.kind,item.item_id,item.version,item.version+1,decision,a,c);
 RETURN public.statement_store_command_v1(t,a,'review',k,c,fp,target_run);END $$;

GRANT USAGE ON SCHEMA public TO ledgersync_statement_owner;
GRANT SELECT ON accounts,ledger_postings,journal_transactions,tenant_subject_roles TO ledgersync_statement_owner;
GRANT SELECT,INSERT ON statement_mappings,statement_imports,statement_rows,statement_snapshot_items,statement_matches,statement_review_events,statement_commands TO ledgersync_statement_owner;
GRANT SELECT,INSERT,UPDATE ON statement_runs,statement_review_items TO ledgersync_statement_owner;
  -- Row locks require UPDATE privilege; append-only triggers still forbid mutation.
  GRANT UPDATE(id) ON statement_mappings TO ledgersync_statement_owner;
GRANT EXECUTE ON FUNCTION tenant_scope_allows(uuid),controlled_append_audit_v1(uuid,uuid,text,text,text,text,text,uuid,jsonb,timestamptz) TO ledgersync_statement_owner;
DO $$ BEGIN IF to_regrole('ledgersync_api') IS NOT NULL THEN GRANT SELECT ON statement_mappings,statement_imports,statement_rows,statement_runs,statement_snapshot_items,statement_matches,statement_review_items,statement_review_events TO ledgersync_api; END IF;END $$;
DO $$ DECLARE signature text;BEGIN
 FOREACH signature IN ARRAY ARRAY['protect_statement_run_v1()','protect_statement_review_v1()','statement_existing_command_v1(uuid,text,text,text,uuid,bytea)','statement_store_command_v1(uuid,text,text,text,uuid,bytea,uuid)',
 'controlled_statement_mapping_v1(uuid,text,text,uuid,uuid,text)','controlled_statement_import_v1(uuid,text,text,uuid,uuid,bytea)',
 'controlled_statement_prepare_v1(uuid,text,text,uuid,uuid)','controlled_statement_complete_v1(uuid,text,uuid,jsonb)','controlled_statement_review_v1(uuid,text,text,uuid,uuid,jsonb)'] LOOP
  EXECUTE format('ALTER FUNCTION public.%s OWNER TO ledgersync_statement_owner',signature);
  EXECUTE format('REVOKE ALL ON FUNCTION public.%s FROM PUBLIC',signature);
  IF signature LIKE 'controlled_%' AND to_regrole('ledgersync_api') IS NOT NULL THEN EXECUTE format('GRANT EXECUTE ON FUNCTION public.%s TO ledgersync_api',signature);END IF;
 END LOOP;
END $$;
REVOKE ALL ON FUNCTION protect_statement_run_v1(),protect_statement_review_v1() FROM PUBLIC;
