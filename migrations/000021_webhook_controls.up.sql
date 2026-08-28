-- Tenant-owned webhook endpoints. Verification challenges are stored only as
-- SHA-256 digests and signing material remains in the external key manager.

CREATE TABLE developer_webhook_endpoints (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 100),
  endpoint_url TEXT NOT NULL CHECK (char_length(endpoint_url) BETWEEN 8 AND 2048),
  subscribed_events TEXT[] NOT NULL CHECK (cardinality(subscribed_events) BETWEEN 1 AND 32),
  signing_key_reference TEXT NOT NULL CHECK (signing_key_reference ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{2,199}$'),
  signing_key_id TEXT NOT NULL CHECK (signing_key_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{2,99}$'),
  status TEXT NOT NULL CHECK (status IN ('pending_verification','active','disabled')),
  version BIGINT NOT NULL CHECK (version > 0),
  challenge_digest BYTEA CHECK (challenge_digest IS NULL OR octet_length(challenge_digest)=32),
  challenge_expires_at TIMESTAMPTZ,
  verified_at TIMESTAMPTZ,
  disabled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id,endpoint_url),
  UNIQUE (id,tenant_id),
  CHECK ((status='pending_verification' AND challenge_digest IS NOT NULL AND challenge_expires_at IS NOT NULL AND verified_at IS NULL AND disabled_at IS NULL)
      OR (status='active' AND challenge_digest IS NULL AND challenge_expires_at IS NULL AND verified_at IS NOT NULL AND disabled_at IS NULL)
      OR (status='disabled' AND challenge_digest IS NULL AND challenge_expires_at IS NULL AND disabled_at IS NOT NULL))
);
CREATE INDEX developer_webhook_endpoints_tenant_updated_idx ON developer_webhook_endpoints (tenant_id,updated_at DESC,id DESC);

CREATE TABLE developer_webhook_events (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  webhook_id UUID NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('registered','verified','signature_rotated','disabled')),
  version BIGINT NOT NULL CHECK (version > 0),
  actor_subject_id TEXT NOT NULL,
  correlation_id UUID NOT NULL,
  sanitized_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (webhook_id,tenant_id) REFERENCES developer_webhook_endpoints(id,tenant_id),
  UNIQUE (webhook_id,version,action)
);
CREATE INDEX developer_webhook_events_tenant_time_idx ON developer_webhook_events (tenant_id,created_at DESC,id DESC);
CREATE TRIGGER developer_webhook_events_append_only BEFORE UPDATE OR DELETE ON developer_webhook_events FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE TABLE developer_webhook_command_idempotency (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL,
  operation TEXT NOT NULL CHECK (operation IN ('webhook_register','webhook_verify','webhook_signature_rotate','webhook_disable')),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 255),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
  state TEXT NOT NULL CHECK (state IN ('in_progress','completed')),
  response_body JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  PRIMARY KEY (tenant_id,actor_subject_id,operation,idempotency_key),
  CHECK ((state='in_progress' AND response_body IS NULL AND completed_at IS NULL) OR (state='completed' AND response_body IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE INDEX developer_webhook_command_expiry_idx ON developer_webhook_command_idempotency (created_at);

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT ON developer_webhook_endpoints,developer_webhook_events,developer_webhook_command_idempotency TO ledgersync_api';
    EXECUTE 'GRANT UPDATE ON developer_webhook_endpoints,developer_webhook_command_idempotency TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_worker') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON developer_webhook_endpoints TO ledgersync_worker';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON developer_webhook_endpoints,developer_webhook_events TO ledgersync_support_readonly';
  END IF;
END $$;
