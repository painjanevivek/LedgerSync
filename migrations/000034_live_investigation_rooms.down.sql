DROP TABLE IF EXISTS investigation_live_room_operations;
DROP TABLE IF EXISTS investigation_live_room_findings;
DROP TABLE IF EXISTS investigation_live_rooms;

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

DROP INDEX IF EXISTS idempotency_requests_tenant_request_reference_idx;
ALTER TABLE idempotency_requests DROP COLUMN IF EXISTS request_reference;
