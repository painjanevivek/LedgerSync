DROP TABLE IF EXISTS investigation_live_room_operations;
DROP TABLE IF EXISTS investigation_live_room_findings;
DROP TABLE IF EXISTS investigation_live_rooms;

DROP FUNCTION IF EXISTS public.controlled_attach_transfer_request_reference_v1(UUID,TEXT,TEXT,BYTEA,UUID);

DELETE FROM investigation_workspace_references WHERE record_type='transfer_request';
DELETE FROM investigation_workspaces WHERE root_record_type='transfer_request' OR query_kind='request_reference';

ALTER TABLE investigation_workspace_references DROP CONSTRAINT investigation_workspace_references_record_type_check;
ALTER TABLE investigation_workspace_references ADD CONSTRAINT investigation_workspace_references_record_type_check
  CHECK (record_type IN ('account','transfer','funding','event','reconciliation_run','reconciliation_mismatch','correction'));
ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_root_record_type_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_root_record_type_check
  CHECK (root_record_type IN ('account','transfer','funding','event','reconciliation_run','reconciliation_mismatch','correction'));
ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_query_kind_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_query_kind_check
  CHECK (query_kind IN ('immutable_id','approved_reference'));

-- Restore the prior guard before removing the column referenced by its
-- first-attachment exception. This file is not a record-preserving rollback.
CREATE OR REPLACE FUNCTION public.protect_resolved_idempotency()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR OLD.state IN ('completed','failed') THEN
    RAISE EXCEPTION 'resolved idempotency outcomes are immutable' USING ERRCODE='55000';
  END IF;
  IF NEW.tenant_id<>OLD.tenant_id OR NEW.actor_subject_id<>OLD.actor_subject_id OR
     NEW.operation<>OLD.operation OR NEW.idempotency_key<>OLD.idempotency_key OR
     NEW.request_fingerprint<>OLD.request_fingerprint OR NEW.state NOT IN ('completed','failed') THEN
    RAISE EXCEPTION 'invalid idempotency finalization' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;
DROP INDEX IF EXISTS idempotency_requests_tenant_request_reference_idx;
ALTER TABLE idempotency_requests DROP COLUMN IF EXISTS request_reference;
