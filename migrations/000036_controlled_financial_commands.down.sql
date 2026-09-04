-- Safe only while the expand-phase application still uses direct DML. Once
-- PR-009 revokes those grants this function must be repaired forward instead.
REVOKE ALL ON FUNCTION controlled_submit_transfer_v1(UUID,TEXT,UUID,UUID,BIGINT,TEXT,TEXT,BYTEA,UUID,TEXT,TIMESTAMPTZ) FROM PUBLIC;
DROP FUNCTION controlled_submit_transfer_v1(UUID,TEXT,UUID,UUID,BIGINT,TEXT,TEXT,BYTEA,UUID,TEXT,TIMESTAMPTZ);
