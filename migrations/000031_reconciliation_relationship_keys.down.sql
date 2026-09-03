DROP INDEX IF EXISTS transfer_corrections_tenant_compensation_idx;
DROP INDEX IF EXISTS outbox_events_tenant_transfer_relation_idx;
DROP INDEX IF EXISTS outbox_events_tenant_account_relation_idx;
DROP INDEX IF EXISTS journal_transactions_tenant_funding_idx;
DROP INDEX IF EXISTS reconciliation_mismatches_tenant_transfer_idx;
ALTER TABLE reconciliation_mismatches
  DROP CONSTRAINT IF EXISTS reconciliation_mismatch_subject_key_check,
  DROP CONSTRAINT IF EXISTS reconciliation_mismatch_transfer_tenant_fk,
  DROP COLUMN IF EXISTS transfer_id;
