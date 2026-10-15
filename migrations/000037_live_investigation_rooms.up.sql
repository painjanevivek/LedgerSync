-- Live investigation rooms are durable lifecycle records around an uncertain
-- transfer request. WebRTC chat, SDP, ICE, presence and submitted intent stay
-- outside PostgreSQL.
-- A preserved local database can already contain the earlier 000034/000035
-- live-room implementation. Accept only that reviewed structure, retain its
-- records/history, and install the missing current controls. The exact legacy
-- definitions used by this preflight are in the integration test fixture.

DO $legacy_preflight$
DECLARE
  legacy_rooms BOOLEAN;
  legacy_compatibility BOOLEAN;
  existing_live_objects BOOLEAN;
  expected RECORD;
  actual_fingerprint TEXT;
BEGIN
  SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='000034_live_investigation_rooms.up.sql'),
         EXISTS(SELECT 1 FROM schema_migrations WHERE version='000035_live_investigation_compatibility.up.sql')
    INTO legacy_rooms,legacy_compatibility;
  existing_live_objects :=
    EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='public.idempotency_requests'::regclass AND attname='request_reference' AND NOT attisdropped)
    OR to_regclass('public.investigation_live_rooms') IS NOT NULL
    OR to_regclass('public.investigation_live_room_findings') IS NOT NULL
    OR to_regclass('public.investigation_live_room_operations') IS NOT NULL
    OR to_regclass('public.idempotency_requests_tenant_request_reference_idx') IS NOT NULL
    OR to_regclass('public.investigation_live_rooms_active_workspace_idx') IS NOT NULL
    OR to_regclass('public.investigation_live_rooms_active_request_idx') IS NOT NULL
    OR to_regclass('public.investigation_live_rooms_peer_idx') IS NOT NULL
    OR to_regprocedure('public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)') IS NOT NULL;
  IF legacy_rooms<>legacy_compatibility OR existing_live_objects<>(legacy_rooms AND legacy_compatibility) THEN
    RAISE EXCEPTION 'incompatible live-room migration state: known legacy history and objects must agree';
  END IF;
  IF NOT existing_live_objects THEN RETURN; END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
    WHERE a.attrelid='public.idempotency_requests'::regclass AND a.attname='request_reference'
      AND a.atttypid='uuid'::regtype AND NOT a.attnotnull AND NOT a.attisdropped AND d.oid IS NULL
  ) OR to_regprocedure('public.controlled_attach_transfer_request_reference_v1(uuid,text,text,bytea,uuid)') IS NOT NULL
    OR to_regclass('public.idempotency_requests_tenant_request_reference_idx') IS NOT NULL THEN
    RAISE EXCEPTION 'incompatible legacy transfer request reference definition';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_index i JOIN pg_class index_relation ON index_relation.oid=i.indexrelid
    WHERE i.indrelid='public.idempotency_requests'::regclass
      AND index_relation.relname='idempotency_request_reference_unique'
      AND i.indisunique AND i.indisvalid AND i.indisready
      AND pg_get_indexdef(i.indexrelid)='CREATE UNIQUE INDEX idempotency_request_reference_unique ON public.idempotency_requests USING btree (tenant_id, request_reference)'
  ) THEN
    RAISE EXCEPTION 'incompatible legacy request reference uniqueness';
  END IF;
  FOR expected IN SELECT * FROM (VALUES
    ('investigation_live_rooms','e27d0f51447c2ccd406d6166b31a66a2'),
    ('investigation_live_room_findings','6df40f2c6cebe82f9e1347deadb39b39'),
    ('investigation_live_room_operations','cec21298e6a9692235756c34591dda2f')
  ) AS definitions(table_name,structural_fingerprint) LOOP
    IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('public.'||expected.table_name) AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
      OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=to_regclass('public.'||expected.table_name) OR inhparent=to_regclass('public.'||expected.table_name))
      OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=to_regclass('public.'||expected.table_name)) THEN
      RAISE EXCEPTION 'incompatible legacy live-room relation: %',expected.table_name;
    END IF;
    IF EXISTS(SELECT 1 FROM pg_attribute a JOIN pg_type t ON t.oid=a.atttypid WHERE a.attrelid=('public.'||expected.table_name)::regclass AND a.attnum>0 AND NOT a.attisdropped AND (a.attgenerated<>'' OR a.attidentity<>'' OR a.attcollation<>t.typcollation))
      OR EXISTS(SELECT 1 FROM pg_index WHERE indrelid=('public.'||expected.table_name)::regclass AND NOT indisready) THEN
      RAISE EXCEPTION 'incompatible legacy generated column or unready index: %',expected.table_name;
    END IF;
    -- These fingerprints compare catalog definitions, not row values or ACLs.
    -- They reject missing/extra columns, wrong nullability/defaults, weakened
    -- keys/checks, and incompatible or invalid indexes. ACLs are applied below.
    SELECT md5(jsonb_build_object(
      'columns',(SELECT jsonb_agg(jsonb_build_array(a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,pg_get_expr(d.adbin,d.adrelid)) ORDER BY a.attname)
        FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
        WHERE a.attrelid=('public.'||expected.table_name)::regclass AND a.attnum>0 AND NOT a.attisdropped),
      'constraints',(SELECT jsonb_agg(jsonb_build_array(c.conname,c.contype,c.convalidated,pg_get_constraintdef(c.oid)) ORDER BY c.conname)
        FROM pg_constraint c WHERE c.conrelid=('public.'||expected.table_name)::regclass),
      'indexes',(SELECT jsonb_agg(jsonb_build_array(i.relname,x.indisvalid,pg_get_indexdef(i.oid)) ORDER BY i.relname)
        FROM pg_index x JOIN pg_class i ON i.oid=x.indexrelid WHERE x.indrelid=('public.'||expected.table_name)::regclass)
      )::text) INTO actual_fingerprint;
    IF actual_fingerprint IS DISTINCT FROM expected.structural_fingerprint THEN
      RAISE EXCEPTION 'incompatible legacy live-room definition: %',expected.table_name;
    END IF;
  END LOOP;
  IF (SELECT count(*) FROM pg_trigger WHERE tgrelid IN ('public.investigation_live_rooms'::regclass,'public.investigation_live_room_findings'::regclass,'public.investigation_live_room_operations'::regclass) AND NOT tgisinternal)<>1
    OR NOT EXISTS (
      SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_language l ON l.oid=p.prolang
      WHERE t.tgrelid='public.investigation_live_room_findings'::regclass
        AND t.tgname='investigation_live_room_findings_immutable_update' AND NOT t.tgisinternal
        AND t.tgenabled='O' AND t.tgtype=27 AND t.tgnargs=0 AND t.tgqual IS NULL
        AND p.oid=to_regprocedure('public.reject_live_room_finding_mutation()')
        AND l.lanname='plpgsql' AND NOT p.prosecdef AND p.proconfig IS NULL
        AND btrim(regexp_replace(p.prosrc,'[[:space:]]+',' ','g'))='BEGIN RAISE EXCEPTION ''live investigation findings are append-only''; END'
    ) THEN
    RAISE EXCEPTION 'incompatible legacy immutable-finding trigger';
  END IF;
END;
$legacy_preflight$;

ALTER TABLE idempotency_requests ADD COLUMN IF NOT EXISTS request_reference UUID;
CREATE UNIQUE INDEX IF NOT EXISTS idempotency_requests_tenant_request_reference_idx
  ON idempotency_requests (tenant_id,request_reference)
  WHERE request_reference IS NOT NULL;

-- A request reference is correlation metadata, attached in the same financial
-- transaction after outcome finalization. Permit its first attachment only;
-- every other field and every already attached reference remains immutable.
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

CREATE FUNCTION public.controlled_attach_transfer_request_reference_v1(
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
REVOKE ALL ON FUNCTION public.controlled_attach_transfer_request_reference_v1(UUID,TEXT,TEXT,BYTEA,UUID) FROM PUBLIC;

ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_query_kind_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_query_kind_check
  CHECK (query_kind IN ('immutable_id','approved_reference','request_reference'));
ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_root_record_type_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_root_record_type_check
  CHECK (root_record_type IN ('account','transfer','transfer_request','funding','event','reconciliation_run','reconciliation_mismatch','correction'));
ALTER TABLE investigation_workspace_references DROP CONSTRAINT investigation_workspace_references_record_type_check;
ALTER TABLE investigation_workspace_references ADD CONSTRAINT investigation_workspace_references_record_type_check
  CHECK (record_type IN ('account','transfer','transfer_request','funding','event','reconciliation_run','reconciliation_mismatch','correction'));

CREATE TABLE IF NOT EXISTS investigation_live_rooms (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  investigation_id UUID NOT NULL,
  transfer_request_reference UUID NOT NULL,
  owner_subject_id TEXT NOT NULL CHECK (length(owner_subject_id) BETWEEN 1 AND 255 AND owner_subject_id=btrim(owner_subject_id) AND owner_subject_id !~ '[[:cntrl:]]'),
  peer_subject_id TEXT CHECK (peer_subject_id IS NULL OR (length(peer_subject_id) BETWEEN 1 AND 255 AND peer_subject_id=btrim(peer_subject_id) AND peer_subject_id !~ '[[:cntrl:]]' AND peer_subject_id<>owner_subject_id)),
  status TEXT NOT NULL CHECK (status IN ('waiting','active','ended','expired')),
  invite_token_hash BYTEA,
  invite_expires_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ,
  FOREIGN KEY (investigation_id,tenant_id) REFERENCES investigation_workspaces(id,tenant_id),
  CHECK (expires_at=created_at+interval '30 minutes'),
  CHECK ((status IN ('waiting','active') AND ended_at IS NULL) OR (status IN ('ended','expired') AND ended_at IS NOT NULL)),
  CHECK ((invite_token_hash IS NULL AND invite_expires_at IS NULL) OR (invite_token_hash IS NOT NULL AND invite_expires_at IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS investigation_live_rooms_active_workspace_idx
  ON investigation_live_rooms (tenant_id,investigation_id)
  WHERE status IN ('waiting','active');
CREATE UNIQUE INDEX IF NOT EXISTS investigation_live_rooms_active_request_idx
  ON investigation_live_rooms (tenant_id,transfer_request_reference)
  WHERE status IN ('waiting','active');
CREATE INDEX IF NOT EXISTS investigation_live_rooms_peer_idx
  ON investigation_live_rooms (tenant_id,peer_subject_id,created_at DESC)
  WHERE peer_subject_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS investigation_live_room_findings (
  room_id UUID PRIMARY KEY REFERENCES investigation_live_rooms(id),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  finding_code TEXT NOT NULL CHECK (finding_code IN ('confirmed_completed','confirmed_rejected','still_unresolved','escalated')),
  authoritative_request_state TEXT NOT NULL CHECK (authoritative_request_state IN ('in_progress','completed','rejected','unavailable','expired')),
  canonical_transfer_id UUID REFERENCES transfers(id),
  actor_subject_id TEXT NOT NULL CHECK (length(actor_subject_id) BETWEEN 1 AND 255 AND actor_subject_id=btrim(actor_subject_id) AND actor_subject_id !~ '[[:cntrl:]]'),
  recorded_at TIMESTAMPTZ NOT NULL,
  CHECK ((finding_code='confirmed_completed' AND authoritative_request_state='completed' AND canonical_transfer_id IS NOT NULL)
      OR (finding_code='confirmed_rejected' AND authoritative_request_state='rejected' AND canonical_transfer_id IS NULL)
      OR (finding_code IN ('still_unresolved','escalated') AND authoritative_request_state IN ('in_progress','unavailable','expired') AND canonical_transfer_id IS NULL))
);

CREATE TABLE IF NOT EXISTS investigation_live_room_operations (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL CHECK (length(actor_subject_id) BETWEEN 1 AND 255),
  operation TEXT NOT NULL CHECK (operation IN ('start','join','end','finding')),
  idempotency_key_hash BYTEA NOT NULL CHECK (octet_length(idempotency_key_hash)=32),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
  room_id UUID NOT NULL REFERENCES investigation_live_rooms(id),
  response_body JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id,actor_subject_id,operation,idempotency_key_hash)
);

DO $legacy_controls$
BEGIN
  IF EXISTS(SELECT 1 FROM schema_migrations WHERE version='000034_live_investigation_rooms.up.sql') THEN
    -- Validate before recording 000037. A historical violation stops the whole
    -- migration. Never rewrite a room, finding, operation, or older version.
    ALTER TABLE investigation_live_rooms ADD CONSTRAINT investigation_live_rooms_current_expiry_check
      CHECK (expires_at=created_at+interval '30 minutes') NOT VALID;
    ALTER TABLE investigation_live_rooms VALIDATE CONSTRAINT investigation_live_rooms_current_expiry_check;
    ALTER TABLE investigation_live_rooms ADD CONSTRAINT investigation_live_rooms_current_invite_pair_check
      CHECK ((invite_token_hash IS NULL AND invite_expires_at IS NULL) OR (invite_token_hash IS NOT NULL AND invite_expires_at IS NOT NULL)) NOT VALID;
    ALTER TABLE investigation_live_rooms VALIDATE CONSTRAINT investigation_live_rooms_current_invite_pair_check;
    ALTER TABLE investigation_live_room_findings VALIDATE CONSTRAINT investigation_live_room_findings_authoritative_check;
  END IF;
END;
$legacy_controls$;

REVOKE ALL ON investigation_live_rooms,investigation_live_room_findings,investigation_live_room_operations FROM PUBLIC;
DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION public.controlled_attach_transfer_request_reference_v1(UUID,TEXT,TEXT,BYTEA,UUID) TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON investigation_live_rooms TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_live_room_findings TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_live_room_operations TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON investigation_live_rooms,investigation_live_room_findings TO ledgersync_support_readonly';
  END IF;
END $$;
