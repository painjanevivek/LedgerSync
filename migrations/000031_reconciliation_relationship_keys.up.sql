-- Relationship navigation must follow explicit, tenant-scoped keys. Historical
-- transfer mismatch references were written by the reconciliation worker into
-- sanitized_details; promote only the two known transfer classifications into
-- a real foreign key so investigation reads never depend on JSON heuristics.

ALTER TABLE reconciliation_mismatches
  ADD COLUMN transfer_id UUID;

-- The migration runner applies this file atomically with a schema-privileged
-- identity. Disable only the named append-only trigger for the bounded legacy
-- promotion, then restore it before installing the new constraints. CASE is
-- used so malformed historical JSON can never reach a UUID cast.
ALTER TABLE reconciliation_mismatches
  DISABLE TRIGGER reconciliation_mismatches_append_only;

WITH candidates AS (
  SELECT
    mismatch.id,
    mismatch.tenant_id,
    CASE
      WHEN mismatch.sanitized_details->>'transfer_id' ~
        '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      THEN (mismatch.sanitized_details->>'transfer_id')::uuid
    END AS transfer_id
  FROM reconciliation_mismatches mismatch
  WHERE mismatch.classification IN ('posted_transfer_incomplete','journal_unbalanced')
)
UPDATE reconciliation_mismatches mismatch
SET transfer_id=candidate.transfer_id
FROM candidates candidate
JOIN transfers transfer
  ON transfer.id=candidate.transfer_id
 AND transfer.tenant_id=candidate.tenant_id
WHERE mismatch.id=candidate.id;

ALTER TABLE reconciliation_mismatches
  ENABLE TRIGGER reconciliation_mismatches_append_only;

ALTER TABLE reconciliation_mismatches
  ADD CONSTRAINT reconciliation_mismatch_transfer_tenant_fk
  FOREIGN KEY (transfer_id,tenant_id) REFERENCES transfers(id,tenant_id);

ALTER TABLE reconciliation_mismatches
  ADD CONSTRAINT reconciliation_mismatch_subject_key_check CHECK (
    (classification IN ('posted_transfer_incomplete','journal_unbalanced') AND transfer_id IS NOT NULL AND account_id IS NULL)
    OR
    (classification NOT IN ('posted_transfer_incomplete','journal_unbalanced') AND transfer_id IS NULL)
  );

CREATE INDEX reconciliation_mismatches_tenant_transfer_idx
  ON reconciliation_mismatches (tenant_id,transfer_id,created_at DESC,id DESC)
  WHERE transfer_id IS NOT NULL;

CREATE INDEX journal_transactions_tenant_funding_idx
  ON journal_transactions (tenant_id,funding_event_id)
  WHERE funding_event_id IS NOT NULL;

CREATE INDEX outbox_events_tenant_account_relation_idx
  ON outbox_events (tenant_id,account_id,occurred_at DESC,id DESC)
  WHERE account_id IS NOT NULL;

CREATE INDEX outbox_events_tenant_transfer_relation_idx
  ON outbox_events (tenant_id,transfer_id,occurred_at DESC,id DESC)
  WHERE transfer_id IS NOT NULL;

CREATE INDEX transfer_corrections_tenant_compensation_idx
  ON transfer_corrections (tenant_id,compensation_transfer_id,updated_at DESC,id DESC)
  WHERE compensation_transfer_id IS NOT NULL;

DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT (transfer_id) ON reconciliation_mismatches TO ledgersync_api';
  END IF;
END $$;
