-- Bounded tenant evidence reads use deterministic keyset ordering and direct
-- event-to-delivery lookup without scanning unrelated tenants.
CREATE INDEX outbox_events_tenant_occurred_idx
  ON outbox_events (tenant_id,occurred_at DESC,id DESC);
CREATE INDEX delivery_attempts_outbox_idx
  ON delivery_attempts (tenant_id,outbox_event_id,attempt_number,id)
  WHERE outbox_event_id IS NOT NULL;
