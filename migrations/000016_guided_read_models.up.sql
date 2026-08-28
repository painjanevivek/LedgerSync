-- Guided local evidence remains read-only. These indexes bound the only new
-- actor/transfer lookups, and existing workload roles receive only the SELECT
-- capabilities required by the additive read model.
CREATE INDEX idempotency_transfer_evidence_idx
  ON idempotency_requests (tenant_id,transfer_id,COALESCE(completed_at,created_at),operation)
  WHERE transfer_id IS NOT NULL;

CREATE INDEX audit_guidance_actor_event_idx
  ON audit_events (tenant_id,actor_subject_id,event_type,occurred_at DESC,id DESC);

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON outbox_events,audit_events TO ledgersync_api';
  END IF;
END $$;
