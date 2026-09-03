-- Saved operational views are server-owned operator preferences. They retain
-- only a versioned, allowlisted filter definition and never snapshot financial
-- results, cursors, credentials, payloads, or arbitrary browser URLs.

CREATE TABLE investigation_saved_views (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  owner_subject_id TEXT NOT NULL CHECK (
    length(owner_subject_id) BETWEEN 1 AND 255
    AND owner_subject_id !~ '[[:cntrl:]]'
  ),
  name TEXT NOT NULL CHECK (
    char_length(name) BETWEEN 1 AND 80
    AND name=btrim(name)
    AND name !~ '[[:cntrl:]]'
  ),
  filter_schema_version SMALLINT NOT NULL CHECK (filter_schema_version=1),
  domain TEXT NOT NULL CHECK (domain IN (
    'accounts','transfers','funding','approvals','corrections','events','webhooks'
  )),
  filters JSONB NOT NULL CHECK (
    jsonb_typeof(filters)='object'
    AND filters<>'{}'::jsonb
    AND octet_length(filters::text)<=4096
  ),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (id,tenant_id)
);

CREATE UNIQUE INDEX investigation_saved_views_owner_name_idx
  ON investigation_saved_views (tenant_id,owner_subject_id,lower(name));

CREATE INDEX investigation_saved_views_owner_recent_idx
  ON investigation_saved_views (tenant_id,owner_subject_id,updated_at DESC,id DESC);

REVOKE ALL ON investigation_saved_views FROM PUBLIC;

DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE,DELETE ON investigation_saved_views TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON investigation_saved_views TO ledgersync_support_readonly';
  END IF;
END $$;
