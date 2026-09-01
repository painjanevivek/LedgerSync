-- Investigation workspaces preserve navigation context and immutable action
-- history. They never snapshot balances, amounts, payloads, credentials, or
-- free-form notes; current financial evidence is re-read from source tables.

CREATE TABLE investigation_workspaces (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  owner_subject_id TEXT NOT NULL CHECK (
    length(owner_subject_id) BETWEEN 1 AND 255
    AND owner_subject_id=btrim(owner_subject_id)
    AND owner_subject_id !~ '[[:cntrl:]]'
  ),
  title TEXT NOT NULL CHECK (
    char_length(title) BETWEEN 1 AND 80
    AND title=btrim(title)
    AND title !~ '[[:cntrl:]<>]'
  ),
  taxonomy TEXT NOT NULL CHECK (taxonomy IN (
    'account_state','transfer_delivery','funding','reconciliation','correction','other'
  )),
  status TEXT NOT NULL CHECK (status IN ('open','closed')),
  query_kind TEXT NOT NULL CHECK (query_kind IN ('immutable_id','approved_reference')),
  root_record_type TEXT NOT NULL CHECK (root_record_type IN (
    'account','transfer','funding','event','reconciliation_run','reconciliation_mismatch','correction'
  )),
  query_value TEXT NOT NULL CHECK (
    char_length(query_value) BETWEEN 8 AND 128
    AND query_value=btrim(query_value)
    AND query_value !~ '[[:cntrl:]]'
  ),
  root_record_id UUID NOT NULL,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  closed_at TIMESTAMPTZ,
  CHECK ((status='open' AND closed_at IS NULL) OR (status='closed' AND closed_at IS NOT NULL)),
  UNIQUE (id,tenant_id)
);

CREATE TABLE investigation_workspace_references (
  investigation_id UUID NOT NULL,
  tenant_id UUID NOT NULL,
  position SMALLINT NOT NULL CHECK (position BETWEEN 0 AND 20),
  relationship_type TEXT NOT NULL CHECK (relationship_type ~ '^[a-z][a-z0-9_]{1,63}$'),
  source_record_type TEXT,
  source_record_id UUID,
  record_type TEXT NOT NULL CHECK (record_type IN (
    'account','transfer','funding','event','reconciliation_run','reconciliation_mismatch','correction'
  )),
  record_id UUID NOT NULL,
  captured_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (investigation_id,position),
  UNIQUE (investigation_id,record_type,record_id),
  FOREIGN KEY (investigation_id,tenant_id) REFERENCES investigation_workspaces(id,tenant_id),
  CHECK (
    (position=0 AND relationship_type='root' AND source_record_type IS NULL AND source_record_id IS NULL)
    OR
    (position>0 AND relationship_type<>'root' AND source_record_type IS NOT NULL AND source_record_id IS NOT NULL)
  )
);

CREATE INDEX investigation_workspaces_owner_recent_idx
  ON investigation_workspaces (tenant_id,owner_subject_id,updated_at DESC,id DESC);

CREATE INDEX investigation_workspace_references_record_idx
  ON investigation_workspace_references (tenant_id,record_type,record_id);

REVOKE ALL ON investigation_workspaces,investigation_workspace_references FROM PUBLIC;

DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON investigation_workspaces TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_workspace_references TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON investigation_workspaces,investigation_workspace_references TO ledgersync_support_readonly';
  END IF;
END $$;
