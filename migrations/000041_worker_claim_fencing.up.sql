ALTER TABLE outbox_events ADD COLUMN claim_generation bigint NOT NULL DEFAULT 0 CHECK (claim_generation >= 0);
ALTER TABLE webhook_delivery_jobs ADD COLUMN claim_generation bigint NOT NULL DEFAULT 0 CHECK (claim_generation >= 0);
ALTER TABLE webhook_endpoint_verification_jobs ADD COLUMN claim_generation bigint NOT NULL DEFAULT 0 CHECK (claim_generation >= 0);
CREATE TABLE outbox_publication_attempts (
 event_id uuid NOT NULL REFERENCES outbox_events(id),
 claim_generation bigint NOT NULL CHECK (claim_generation > 0),
 worker_id text NOT NULL,
 status text NOT NULL CHECK (status IN ('claimed','published','retrying','dead')),
 started_at timestamptz NOT NULL,
 lease_until timestamptz NOT NULL,
 completed_at timestamptz,
 error_code text,
 PRIMARY KEY (event_id,claim_generation)
);
CREATE INDEX outbox_publication_attempts_worker ON outbox_publication_attempts(worker_id,started_at DESC);
