-- Bind delivery replay approval and execution retries to stable caller-owned
-- request identities. Correlation IDs remain server-generated trace evidence;
-- they are no longer overloaded as a cross-operator command key.

ALTER TABLE delivery_replay_actions
  ADD COLUMN request_key TEXT;

UPDATE delivery_replay_actions
SET request_key=correlation_id::text
WHERE request_key IS NULL;

ALTER TABLE delivery_replay_actions
  ALTER COLUMN request_key SET NOT NULL,
  ADD CONSTRAINT delivery_replay_request_key_bounded
    CHECK (length(request_key) BETWEEN 16 AND 255 AND request_key !~ '[[:space:][:cntrl:]]'),
  ADD CONSTRAINT delivery_replay_request_identity_unique
    UNIQUE (tenant_id,attempt_id,action,request_key);
