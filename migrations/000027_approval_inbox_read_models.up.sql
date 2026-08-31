-- Tenant-first stable read paths for the bounded cross-domain approval inbox.
-- Domain command tables remain the source of truth; these indexes add no
-- mutable approval state and preserve the existing status-specific indexes.

CREATE INDEX funding_events_approval_queue_idx
  ON funding_events (tenant_id, requested_at ASC, id ASC);

CREATE INDEX transfer_corrections_approval_queue_idx
  ON transfer_corrections (tenant_id, requested_at ASC, id ASC);
