-- Additive repair for targets that already recorded the original current
-- 000037/000038. ApplyPending must not replay or rewrite their migration history.
-- Only the two reviewed prior/current definitions are accepted. No financial
-- row, reference, table, index, or workload membership is changed by this DDL.
DO $reference_repair_preflight$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM schema_migrations WHERE version='000037_live_investigation_rooms.up.sql')
     OR NOT EXISTS(SELECT 1 FROM schema_migrations WHERE version='000038_live_investigation_compatibility.up.sql')
     OR NOT EXISTS (
       SELECT 1 FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
       WHERE a.attrelid='public.idempotency_requests'::regclass AND a.attname='request_reference'
         AND a.atttypid='uuid'::regtype AND NOT a.attnotnull AND NOT a.attisdropped
         AND a.attgenerated='' AND a.attidentity='' AND d.oid IS NULL
     ) THEN
    RAISE EXCEPTION 'incompatible transfer reference repair migration state';
  END IF;
  IF (SELECT count(*) FROM pg_trigger WHERE tgrelid='public.idempotency_requests'::regclass AND NOT tgisinternal)<>1
     OR NOT EXISTS (
       SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_language l ON l.oid=p.prolang
       WHERE t.tgrelid='public.idempotency_requests'::regclass
         AND t.tgname='idempotency_final_outcome_immutable' AND NOT t.tgisinternal
         AND t.tgenabled='O' AND t.tgtype=27 AND t.tgnargs=0 AND t.tgqual IS NULL
         AND p.oid=to_regprocedure('public.protect_resolved_idempotency()')
         AND p.prorettype='trigger'::regtype AND l.lanname='plpgsql' AND NOT p.prosecdef
         AND p.proconfig IS NULL AND p.provolatile='v' AND NOT p.proleakproof
         AND NOT p.proisstrict AND NOT p.proretset AND p.prokind='f'
         AND p.proowner=(SELECT relowner FROM pg_class WHERE oid='public.idempotency_requests'::regclass)
         AND md5(btrim(regexp_replace(p.prosrc,'[[:space:]]+',' ','g'))) IN (
           'd7710a4c8d1440c8a9370353c9a3227b', -- original strict 000013 guard
           'b4e86e30c3be70477b203dbe833b1789'  -- corrected 000037 guard
         )
     ) THEN
    RAISE EXCEPTION 'incompatible transfer reference outcome guard';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang
    WHERE p.oid=to_regprocedure('public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)')
      AND p.prorettype='void'::regtype AND l.lanname='plpgsql' AND p.prosecdef
      AND p.proconfig=ARRAY['search_path=pg_catalog, public'] AND p.provolatile='v' AND NOT p.proleakproof
      AND NOT p.proisstrict AND NOT p.proretset AND p.prokind='f'
      AND (p.proowner=(SELECT relowner FROM pg_class WHERE oid='public.idempotency_requests'::regclass)
        OR p.proowner=to_regrole('ledgersync_migration_owner'))
      AND md5(btrim(regexp_replace(p.prosrc,'[[:space:]]+',' ','g'))) IN (
        '0631b61a1f0adeb32698e3a86a4e8fe6', -- original current 000037 helper
        '7f34de9ba390c7686e3bdb34a3761bd6'  -- corrected 000037 helper
      )
  ) THEN
    RAISE EXCEPTION 'incompatible transfer reference attachment helper';
  END IF;
END;
$reference_repair_preflight$;

CREATE OR REPLACE FUNCTION public.protect_resolved_idempotency()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='UPDATE' AND NEW.request_reference IS DISTINCT FROM OLD.request_reference THEN
    IF OLD.operation='transfers.create.v1' AND OLD.state IN ('completed','failed')
       AND OLD.request_reference IS NULL AND NEW.request_reference IS NOT NULL
       AND (to_jsonb(NEW)-'request_reference')=(to_jsonb(OLD)-'request_reference') THEN
      RETURN NEW;
    END IF;
    RAISE EXCEPTION 'idempotency request reference changes must preserve every outcome field' USING ERRCODE='55000';
  END IF;
  IF TG_OP='DELETE' OR OLD.state IN ('completed','failed') THEN
    RAISE EXCEPTION 'resolved idempotency outcomes are immutable' USING ERRCODE='55000';
  END IF;
  IF NEW.tenant_id<>OLD.tenant_id OR NEW.actor_subject_id<>OLD.actor_subject_id OR
     NEW.operation<>OLD.operation OR NEW.idempotency_key<>OLD.idempotency_key OR
     NEW.request_fingerprint<>OLD.request_fingerprint OR NEW.state NOT IN ('completed','failed') THEN
    RAISE EXCEPTION 'invalid idempotency finalization' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.controlled_attach_transfer_request_reference_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_idempotency_key TEXT,
  p_request_fingerprint BYTEA,
  p_request_reference UUID
) RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path=pg_catalog, public
AS $$
BEGIN
  PERFORM 1 FROM public.idempotency_requests
  WHERE tenant_id=p_tenant_id AND actor_subject_id=p_actor_subject_id
    AND operation='transfers.create.v1' AND idempotency_key=p_idempotency_key
    AND request_fingerprint=p_request_fingerprint AND request_reference=p_request_reference;
  IF FOUND THEN RETURN; END IF;
  UPDATE public.idempotency_requests
  SET request_reference=p_request_reference
  WHERE tenant_id=p_tenant_id
    AND actor_subject_id=p_actor_subject_id
    AND operation='transfers.create.v1'
    AND idempotency_key=p_idempotency_key
    AND request_fingerprint=p_request_fingerprint
    AND request_reference IS NULL AND p_request_reference IS NOT NULL;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      ERRCODE='23505',
      CONSTRAINT='controlled_transfer_request_reference',
      MESSAGE='transfer request reference does not match the retained idempotency outcome';
  END IF;
END;
$$;

-- CREATE OR REPLACE preserves existing owners. Canonical roles.sql assigns the
-- helper to the NOLOGIN migration owner and its existing least-privilege object
-- grants before API startup. This migration adds no role or table privilege.
REVOKE ALL ON FUNCTION public.controlled_attach_transfer_request_reference_v1(UUID,TEXT,TEXT,BYTEA,UUID) FROM PUBLIC;
DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_attach_transfer_request_reference_v1(UUID,TEXT,TEXT,BYTEA,UUID) TO ledgersync_api';
  END IF;
END $$;
