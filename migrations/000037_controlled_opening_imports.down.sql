-- Executed opening value is irreversible financial history. It is corrected
-- through an approved compensation/import, never by a destructive migration.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM opening_import_executions) THEN
    RAISE EXCEPTION 'cannot remove opening import controls after an import executed';
  END IF;
END $$;

REVOKE ALL ON FUNCTION controlled_execute_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
DROP FUNCTION controlled_execute_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ);
REVOKE ALL ON FUNCTION controlled_approve_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
DROP FUNCTION controlled_approve_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ);
REVOKE ALL ON FUNCTION controlled_request_opening_import_v1(UUID,TEXT,UUID,TEXT,UUID[],BIGINT[],BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
DROP FUNCTION controlled_request_opening_import_v1(UUID,TEXT,UUID,TEXT,UUID[],BIGINT[],BYTEA,UUID,TIMESTAMPTZ);

DROP INDEX outbox_opening_import_account_idx;
ALTER TABLE outbox_events DROP CONSTRAINT outbox_command_aggregate_consistency;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_command_aggregate_consistency CHECK (
  (aggregate_type='account_balance' AND account_id IS NOT NULL AND aggregate_id=account_id
    AND ((transfer_id IS NOT NULL AND funding_event_id IS NULL) OR (transfer_id IS NULL AND funding_event_id IS NOT NULL)))
  OR (aggregate_type='account' AND transfer_id IS NULL AND funding_event_id IS NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
  OR (aggregate_type='funding_event' AND transfer_id IS NULL AND funding_event_id IS NOT NULL AND aggregate_id=funding_event_id)
  OR (aggregate_type='transfer' AND transfer_id IS NOT NULL AND funding_event_id IS NULL AND account_id IS NULL AND aggregate_id=transfer_id)
  OR (aggregate_type NOT IN ('account_balance','account','funding_event','transfer') AND transfer_id IS NULL AND funding_event_id IS NULL)
);
ALTER TABLE outbox_events DROP COLUMN opening_import_id;

DROP TABLE opening_import_executions;
DROP TABLE opening_import_approvals;
DROP TABLE opening_import_rows;
DROP TABLE opening_import_batches;
