-- Keep the existing public signatures, financial bodies, replay contracts and
-- function ACLs. Add an explicit actor binding to the established commands.
DO $migration$
DECLARE signature text; routine record; tenant_argument text; actor_argument text; guard text; guarded_body text; definition text; error_constraint text;
BEGIN
 FOREACH signature IN ARRAY ARRAY[
  'public.controlled_submit_transfer_v1(uuid,text,uuid,uuid,bigint,text,text,bytea,uuid,text,timestamptz)',
  'public.controlled_post_funding_v1(uuid,text,uuid,text,uuid,timestamptz)',
  'public.controlled_post_transfer_correction_v1(uuid,text,uuid,text,uuid,timestamptz,timestamptz)',
  'public.controlled_provision_account_v1(uuid,text,uuid,text,text,text,text,text[],text[],text[],uuid,timestamptz)',
  'public.controlled_prepare_transfer_recovery_v1(uuid,text,uuid,text,uuid,uuid,text,bigint,bytea)',
  'public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)'
 ] LOOP
  SELECT p.*,l.lanname INTO STRICT routine FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang WHERE p.oid=signature::regprocedure;
  IF routine.lanname<>'plpgsql' OR strpos(routine.prosrc,'BEGIN')=0 OR routine.proargnames[1] NOT IN ('p_tenant_id','p_tenant')
   OR routine.proargnames[2] NOT IN ('p_actor_subject_id','p_actor') THEN RAISE EXCEPTION 'unexpected controlled command shape: %',signature; END IF;
  tenant_argument:=routine.proargnames[1];actor_argument:=routine.proargnames[2];
  error_constraint:=CASE WHEN routine.proname LIKE '%funding%' THEN 'controlled_funding_actor' WHEN routine.proname LIKE '%correction%' THEN 'controlled_correction_actor'
   WHEN routine.proname LIKE '%provision%' THEN 'controlled_account_actor' ELSE 'controlled_transfer_actor' END;
  -- Null envelope probes retain the original input-validation error contract.
  guard:=format(E'BEGIN\n  -- R03: bind the declared command actor to this transaction.\n  IF %1$I IS NOT NULL AND %2$I IS NOT NULL AND (public.tenant_scope_allows(%1$I) IS DISTINCT FROM true OR %2$I IS DISTINCT FROM current_setting(''app.actor_id'',true)) THEN\n    RAISE EXCEPTION ''controlled command scope denied'' USING ERRCODE=''42501'',CONSTRAINT=%3$L;\n  END IF;',tenant_argument,actor_argument,error_constraint);
  guarded_body:=regexp_replace(routine.prosrc,'\mBEGIN\M',guard);
  definition:=replace(pg_get_functiondef(routine.oid),routine.prosrc,guarded_body);
  IF definition=pg_get_functiondef(routine.oid) THEN RAISE EXCEPTION 'controlled command guard was not inserted'; END IF;
  EXECUTE definition;
 END LOOP;
END $migration$;
