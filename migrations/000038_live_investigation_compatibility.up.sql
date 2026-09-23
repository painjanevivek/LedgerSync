-- Migration 000034 was briefly applied by a reverted local implementation
-- with different column names. Preserve those databases by converging them
-- forward; never rewrite or delete the recorded migration.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_rooms' AND column_name='request_reference'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_rooms' AND column_name='transfer_request_reference'
  ) THEN
    ALTER TABLE investigation_live_rooms RENAME COLUMN request_reference TO transfer_request_reference;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_rooms' AND column_name='invite_digest'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_rooms' AND column_name='invite_token_hash'
  ) THEN
    ALTER TABLE investigation_live_rooms RENAME COLUMN invite_digest TO invite_token_hash;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_room_findings' AND column_name='request_status'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_room_findings' AND column_name='authoritative_request_state'
  ) THEN
    ALTER TABLE investigation_live_room_findings RENAME COLUMN request_status TO authoritative_request_state;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_room_findings' AND column_name='transfer_id'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_room_findings' AND column_name='canonical_transfer_id'
  ) THEN
    ALTER TABLE investigation_live_room_findings RENAME COLUMN transfer_id TO canonical_transfer_id;
  END IF;
END $$;
ALTER TABLE investigation_live_rooms ALTER COLUMN invite_token_hash DROP NOT NULL;
ALTER TABLE investigation_live_rooms ALTER COLUMN invite_expires_at DROP NOT NULL;

ALTER TABLE investigation_live_rooms DROP CONSTRAINT IF EXISTS investigation_live_rooms_tenant_id_request_reference_key;
ALTER TABLE investigation_live_rooms DROP CONSTRAINT IF EXISTS investigation_live_rooms_investigation_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS investigation_live_rooms_active_workspace_idx
  ON investigation_live_rooms (tenant_id,investigation_id)
  WHERE status IN ('waiting','active');
CREATE UNIQUE INDEX IF NOT EXISTS investigation_live_rooms_active_request_idx
  ON investigation_live_rooms (tenant_id,transfer_request_reference)
  WHERE status IN ('waiting','active');

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='public' AND table_name='investigation_live_room_findings' AND column_name='id'
  ) THEN
    ALTER TABLE investigation_live_room_findings ALTER COLUMN id SET DEFAULT gen_random_uuid();
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid='investigation_live_room_findings'::regclass
      AND conname='investigation_live_room_findings_authoritative_check'
  ) THEN
    ALTER TABLE investigation_live_room_findings
      ADD CONSTRAINT investigation_live_room_findings_authoritative_check CHECK (
        (finding_code='confirmed_completed' AND authoritative_request_state='completed' AND canonical_transfer_id IS NOT NULL)
        OR (finding_code='confirmed_rejected' AND authoritative_request_state='rejected' AND canonical_transfer_id IS NULL)
        OR (finding_code IN ('still_unresolved','escalated') AND authoritative_request_state IN ('in_progress','unavailable','expired') AND canonical_transfer_id IS NULL)
      ) NOT VALID;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS investigation_live_room_operations (
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
