-- Browser sessions are opaque, revocable server records. Only a SHA-256 digest
-- of the browser bearer is retained; identity, authorization, CSRF, and
-- read-your-writes requirements remain server-side and tenant scoped.
CREATE TABLE bff_sessions (
  token_digest BYTEA PRIMARY KEY CHECK (octet_length(token_digest) = 32),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  subject_id TEXT NOT NULL CHECK (char_length(subject_id) BETWEEN 1 AND 256),
  csrf_token TEXT NOT NULL CHECK (char_length(csrf_token) BETWEEN 16 AND 128),
  roles JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(roles) = 'array' AND jsonb_array_length(roles) <= 16),
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(scopes) = 'array' AND jsonb_array_length(scopes) <= 32),
  consistency_requirements JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(consistency_requirements) = 'object' AND jsonb_array_length(jsonb_path_query_array(consistency_requirements, '$.*')) <= 10),
  authenticated_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (expires_at > created_at),
  CHECK (authenticated_at IS NULL OR authenticated_at <= created_at + interval '30 seconds')
);

CREATE INDEX bff_sessions_expiry_idx ON bff_sessions (expires_at) WHERE revoked_at IS NULL;
CREATE INDEX bff_sessions_operator_idx ON bff_sessions (tenant_id, subject_id, expires_at DESC) WHERE revoked_at IS NULL;

CREATE TABLE operator_ui_preferences (
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  subject_id TEXT NOT NULL CHECK (char_length(subject_id) BETWEEN 1 AND 256),
  experience_mode TEXT NOT NULL DEFAULT 'simple' CHECK (experience_mode IN ('simple', 'expert')),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, subject_id)
);

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE,DELETE ON bff_sessions TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON operator_ui_preferences TO ledgersync_api';
  END IF;
END $$;
