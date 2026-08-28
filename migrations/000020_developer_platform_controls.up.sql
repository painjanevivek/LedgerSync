-- Partner-facing credential metadata and developer command idempotency. Raw
-- credentials remain exclusively in the external identity provider or secret
-- manager and are never accepted by these tables.

CREATE TABLE developer_credentials (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 100),
  external_reference TEXT NOT NULL CHECK (external_reference ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{2,199}$'),
  audience TEXT NOT NULL CHECK (audience ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{2,199}$'),
  scopes TEXT[] NOT NULL CHECK (cardinality(scopes) BETWEEN 1 AND 16),
  status TEXT NOT NULL CHECK (status IN ('active','revoked')),
  version BIGINT NOT NULL CHECK (version > 0),
  expires_at TIMESTAMPTZ NOT NULL,
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  UNIQUE (tenant_id,external_reference),
  UNIQUE (id,tenant_id),
  CHECK ((status='active' AND revoked_at IS NULL) OR (status='revoked' AND revoked_at IS NOT NULL)),
  CHECK (last_used_at IS NULL OR last_used_at >= created_at)
);
CREATE INDEX developer_credentials_tenant_updated_idx ON developer_credentials (tenant_id,updated_at DESC,id DESC);

CREATE TABLE developer_credential_events (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  credential_id UUID NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('created','rotated','revoked')),
  version BIGINT NOT NULL CHECK (version > 0),
  actor_subject_id TEXT NOT NULL,
  correlation_id UUID NOT NULL,
  sanitized_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (credential_id,tenant_id) REFERENCES developer_credentials(id,tenant_id),
  UNIQUE (credential_id,version,action)
);
CREATE INDEX developer_credential_events_tenant_time_idx ON developer_credential_events (tenant_id,created_at DESC,id DESC);
CREATE TRIGGER developer_credential_events_append_only BEFORE UPDATE OR DELETE ON developer_credential_events FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE TABLE developer_command_idempotency (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL,
  operation TEXT NOT NULL CHECK (operation IN ('developer_credential_create','developer_credential_rotate','developer_credential_revoke')),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 255),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
  state TEXT NOT NULL CHECK (state IN ('in_progress','completed')),
  response_body JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  PRIMARY KEY (tenant_id,actor_subject_id,operation,idempotency_key),
  CHECK ((state='in_progress' AND response_body IS NULL AND completed_at IS NULL) OR (state='completed' AND response_body IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE INDEX developer_command_idempotency_expiry_idx ON developer_command_idempotency (created_at);
