-- Delivery leases prevent duplicate active work while retaining the outbox's
-- at-least-once recovery guarantee if a worker dies after publish.
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS claim_owner TEXT,
    ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS dead_at TIMESTAMPTZ;

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_claim_pair_check CHECK (
        (claim_owner IS NULL AND claimed_until IS NULL)
        OR (claim_owner IS NOT NULL AND claimed_until IS NOT NULL)
    );

ALTER TABLE outbox_events
    ADD CONSTRAINT outbox_terminal_state_check CHECK (
        NOT (published_at IS NOT NULL AND dead_at IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS outbox_claimable_idx
    ON outbox_events (available_at, created_at, id)
    WHERE published_at IS NULL AND dead_at IS NULL;
