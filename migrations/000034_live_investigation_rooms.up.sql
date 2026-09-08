-- Live investigation rooms are durable lifecycle records around an uncertain
-- transfer request. WebRTC chat, SDP, ICE, presence and submitted intent stay
-- outside PostgreSQL.

ALTER TABLE idempotency_requests ADD COLUMN request_reference UUID;
CREATE UNIQUE INDEX idempotency_requests_tenant_request_reference_idx
  ON idempotency_requests (tenant_id,request_reference)
  WHERE request_reference IS NOT NULL;

ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_query_kind_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_query_kind_check
  CHECK (query_kind IN ('immutable_id','approved_reference','request_reference'));
ALTER TABLE investigation_workspaces DROP CONSTRAINT investigation_workspaces_root_record_type_check;
ALTER TABLE investigation_workspaces ADD CONSTRAINT investigation_workspaces_root_record_type_check
  CHECK (root_record_type IN ('account','transfer','transfer_request','funding','event','reconciliation_run','reconciliation_mismatch','correction'));
ALTER TABLE investigation_workspace_references DROP CONSTRAINT investigation_workspace_references_record_type_check;
ALTER TABLE investigation_workspace_references ADD CONSTRAINT investigation_workspace_references_record_type_check
  CHECK (record_type IN ('account','transfer','transfer_request','funding','event','reconciliation_run','reconciliation_mismatch','correction'));

CREATE TABLE investigation_live_rooms (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  investigation_id UUID NOT NULL,
  transfer_request_reference UUID NOT NULL,
  owner_subject_id TEXT NOT NULL CHECK (length(owner_subject_id) BETWEEN 1 AND 255 AND owner_subject_id=btrim(owner_subject_id) AND owner_subject_id !~ '[[:cntrl:]]'),
  peer_subject_id TEXT CHECK (peer_subject_id IS NULL OR (length(peer_subject_id) BETWEEN 1 AND 255 AND peer_subject_id=btrim(peer_subject_id) AND peer_subject_id !~ '[[:cntrl:]]' AND peer_subject_id<>owner_subject_id)),
  status TEXT NOT NULL CHECK (status IN ('waiting','active','ended','expired')),
  invite_token_hash BYTEA,
  invite_expires_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ,
  FOREIGN KEY (investigation_id,tenant_id) REFERENCES investigation_workspaces(id,tenant_id),
  CHECK (expires_at=created_at+interval '30 minutes'),
  CHECK ((status IN ('waiting','active') AND ended_at IS NULL) OR (status IN ('ended','expired') AND ended_at IS NOT NULL)),
  CHECK ((invite_token_hash IS NULL AND invite_expires_at IS NULL) OR (invite_token_hash IS NOT NULL AND invite_expires_at IS NOT NULL))
);

CREATE UNIQUE INDEX investigation_live_rooms_active_workspace_idx
  ON investigation_live_rooms (tenant_id,investigation_id)
  WHERE status IN ('waiting','active');
CREATE UNIQUE INDEX investigation_live_rooms_active_request_idx
  ON investigation_live_rooms (tenant_id,transfer_request_reference)
  WHERE status IN ('waiting','active');
CREATE INDEX investigation_live_rooms_peer_idx
  ON investigation_live_rooms (tenant_id,peer_subject_id,created_at DESC)
  WHERE peer_subject_id IS NOT NULL;

CREATE TABLE investigation_live_room_findings (
  room_id UUID PRIMARY KEY REFERENCES investigation_live_rooms(id),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  finding_code TEXT NOT NULL CHECK (finding_code IN ('confirmed_completed','confirmed_rejected','still_unresolved','escalated')),
  authoritative_request_state TEXT NOT NULL CHECK (authoritative_request_state IN ('in_progress','completed','rejected','unavailable','expired')),
  canonical_transfer_id UUID REFERENCES transfers(id),
  actor_subject_id TEXT NOT NULL CHECK (length(actor_subject_id) BETWEEN 1 AND 255 AND actor_subject_id=btrim(actor_subject_id) AND actor_subject_id !~ '[[:cntrl:]]'),
  recorded_at TIMESTAMPTZ NOT NULL,
  CHECK ((finding_code='confirmed_completed' AND authoritative_request_state='completed' AND canonical_transfer_id IS NOT NULL)
      OR (finding_code='confirmed_rejected' AND authoritative_request_state='rejected' AND canonical_transfer_id IS NULL)
      OR (finding_code IN ('still_unresolved','escalated') AND authoritative_request_state IN ('in_progress','unavailable','expired') AND canonical_transfer_id IS NULL))
);

CREATE TABLE investigation_live_room_operations (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL CHECK (length(actor_subject_id) BETWEEN 1 AND 255),
  operation TEXT NOT NULL CHECK (operation IN ('start','join','end','finding')),
  idempotency_key_hash BYTEA NOT NULL CHECK (octet_length(idempotency_key_hash)=32),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
  room_id UUID NOT NULL REFERENCES investigation_live_rooms(id),
  response_body JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id,actor_subject_id,operation,idempotency_key_hash)
);

REVOKE ALL ON investigation_live_rooms,investigation_live_room_findings,investigation_live_room_operations FROM PUBLIC;
DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON investigation_live_rooms TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_live_room_findings TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON investigation_live_room_operations TO ledgersync_api';
  END IF;
  IF to_regrole('ledgersync_support_readonly') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON investigation_live_rooms,investigation_live_room_findings TO ledgersync_support_readonly';
  END IF;
END $$;
