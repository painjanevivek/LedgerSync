-- A BFF actor assertion may be accepted by any API VM. Persist only a digest
-- of its unique ID so replay rejection survives process restarts and horizontal
-- scaling. Rows expire with the assertion and are safely reclaimed in bounded
-- batches by the API path.

CREATE TABLE bff_actor_assertion_replays (
  assertion_digest BYTEA PRIMARY KEY CHECK (octet_length(assertion_digest)=32),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CHECK (expires_at > created_at)
);
CREATE INDEX bff_actor_assertion_replays_expiry_idx
  ON bff_actor_assertion_replays (expires_at);

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,DELETE ON bff_actor_assertion_replays TO ledgersync_api';
  END IF;
END $$;
