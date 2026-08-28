-- Durable webhook jobs are operational state; every completed HTTP outcome is
-- retained separately in append-only delivery_attempts. This deliberately
-- permits at-least-once dispatch while retaining evidence for every observed
-- result and protecting the original event payload from mutation.

-- A canonical transfer event is distinct from an account-balance event: it has
-- exactly one transfer aggregate and no account/funding aggregate. Keep every
-- prior shape explicit so the broader outbox invariant remains fail-closed.
ALTER TABLE outbox_events DROP CONSTRAINT outbox_command_aggregate_consistency;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_command_aggregate_consistency CHECK (
  (aggregate_type='account_balance' AND account_id IS NOT NULL AND aggregate_id=account_id
    AND ((transfer_id IS NOT NULL AND funding_event_id IS NULL) OR (transfer_id IS NULL AND funding_event_id IS NOT NULL)))
  OR (aggregate_type='account' AND transfer_id IS NULL AND funding_event_id IS NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
  OR (aggregate_type='funding_event' AND transfer_id IS NULL AND funding_event_id IS NOT NULL AND aggregate_id=funding_event_id)
  OR (aggregate_type='transfer' AND transfer_id IS NOT NULL AND funding_event_id IS NULL AND account_id IS NULL AND aggregate_id=transfer_id)
  OR (aggregate_type NOT IN ('account_balance','account','funding_event','transfer') AND transfer_id IS NULL AND funding_event_id IS NULL)
);

CREATE TABLE webhook_delivery_jobs (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  transfer_id UUID NOT NULL,
  outbox_event_id UUID NOT NULL REFERENCES outbox_events(id),
  webhook_id UUID NOT NULL,
  event_id UUID NOT NULL,
  event_type TEXT NOT NULL CHECK (event_type IN ('transfer.posted')),
  payload JSONB NOT NULL,
  attempt_number INTEGER NOT NULL DEFAULT 1 CHECK (attempt_number > 0),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','retrying','delivered','dead')),
  available_at TIMESTAMPTZ NOT NULL,
  claim_owner TEXT,
  claimed_until TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  last_error_code TEXT,
  replay_of_attempt_id UUID REFERENCES delivery_attempts(id),
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (transfer_id,tenant_id) REFERENCES transfers(id,tenant_id),
  FOREIGN KEY (webhook_id,tenant_id) REFERENCES developer_webhook_endpoints(id,tenant_id),
  CHECK ((status IN ('pending','retrying') AND completed_at IS NULL)
      OR (status IN ('delivered','dead') AND completed_at IS NOT NULL)),
  CHECK ((claim_owner IS NULL AND claimed_until IS NULL) OR (claim_owner IS NOT NULL AND claimed_until IS NOT NULL))
);
CREATE UNIQUE INDEX webhook_delivery_jobs_initial_event_endpoint_idx
  ON webhook_delivery_jobs (outbox_event_id,webhook_id)
  WHERE replay_of_attempt_id IS NULL;
CREATE UNIQUE INDEX webhook_delivery_jobs_replay_attempt_idx
  ON webhook_delivery_jobs (replay_of_attempt_id)
  WHERE replay_of_attempt_id IS NOT NULL;
CREATE INDEX webhook_delivery_jobs_claim_idx
  ON webhook_delivery_jobs (available_at,created_at,id)
  WHERE status IN ('pending','retrying');
CREATE INDEX webhook_delivery_jobs_transfer_idx
  ON webhook_delivery_jobs (tenant_id,transfer_id,created_at DESC,id DESC);

CREATE OR REPLACE FUNCTION protect_webhook_delivery_job_evidence()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' THEN
    RAISE EXCEPTION 'webhook delivery jobs are retained as operational evidence' USING ERRCODE='55000';
  END IF;
  IF NEW.id<>OLD.id OR NEW.tenant_id<>OLD.tenant_id OR NEW.transfer_id<>OLD.transfer_id OR
     NEW.outbox_event_id<>OLD.outbox_event_id OR NEW.webhook_id<>OLD.webhook_id OR
     NEW.event_id<>OLD.event_id OR NEW.event_type<>OLD.event_type OR NEW.payload<>OLD.payload OR
     NEW.replay_of_attempt_id IS DISTINCT FROM OLD.replay_of_attempt_id OR NEW.created_at<>OLD.created_at THEN
    RAISE EXCEPTION 'webhook delivery job evidence is immutable' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER webhook_delivery_jobs_protect_evidence
BEFORE UPDATE OR DELETE ON webhook_delivery_jobs
FOR EACH ROW EXECUTE FUNCTION protect_webhook_delivery_job_evidence();

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT ON webhook_delivery_jobs TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_worker') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,UPDATE ON webhook_delivery_jobs TO ledgersync_worker';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON webhook_delivery_jobs TO ledgersync_support_readonly';
  END IF;
END $$;
