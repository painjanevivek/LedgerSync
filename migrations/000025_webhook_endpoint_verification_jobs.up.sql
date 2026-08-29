-- Endpoint ownership is proven only by a server-originated challenge. The API
-- writes the encrypted-boundary work item but cannot read it back; the worker
-- can read and advance it after making the hardened outbound request.

ALTER TABLE developer_webhook_events DROP CONSTRAINT IF EXISTS developer_webhook_events_action_check;
ALTER TABLE developer_webhook_events ADD CONSTRAINT developer_webhook_events_action_check
  CHECK (action IN ('registered','verification_scheduled','verified','signature_rotated','disabled'));

CREATE TABLE webhook_endpoint_verification_jobs (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  webhook_id UUID NOT NULL,
  challenge BYTEA NOT NULL CHECK (octet_length(challenge) BETWEEN 32 AND 255),
  expires_at TIMESTAMPTZ NOT NULL,
  attempt_number INTEGER NOT NULL DEFAULT 1 CHECK (attempt_number > 0),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','retrying','verified','dead')),
  available_at TIMESTAMPTZ NOT NULL,
  claim_owner TEXT,
  claimed_until TIMESTAMPTZ,
  last_error_code TEXT,
  completed_at TIMESTAMPTZ,
  correlation_id UUID NOT NULL,
  actor_subject_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (webhook_id,tenant_id) REFERENCES developer_webhook_endpoints(id,tenant_id),
  CHECK ((status IN ('pending','retrying') AND completed_at IS NULL)
      OR (status IN ('verified','dead') AND completed_at IS NOT NULL)),
  CHECK ((claim_owner IS NULL AND claimed_until IS NULL) OR (claim_owner IS NOT NULL AND claimed_until IS NOT NULL))
);
CREATE UNIQUE INDEX webhook_endpoint_verification_one_open_idx
  ON webhook_endpoint_verification_jobs (webhook_id)
  WHERE status IN ('pending','retrying');
CREATE INDEX webhook_endpoint_verification_claim_idx
  ON webhook_endpoint_verification_jobs (available_at,created_at,id)
  WHERE status IN ('pending','retrying');

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT INSERT ON webhook_endpoint_verification_jobs TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_worker') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,UPDATE ON webhook_endpoint_verification_jobs TO ledgersync_worker';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT (id,tenant_id,webhook_id,expires_at,attempt_number,status,available_at,last_error_code,completed_at,created_at,updated_at) ON webhook_endpoint_verification_jobs TO ledgersync_support_readonly';
  END IF;
END $$;
