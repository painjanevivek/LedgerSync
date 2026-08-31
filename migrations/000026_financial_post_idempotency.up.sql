-- Bind retryable browser/BFF post commands to the permanent record they may create.
-- Existing posted records remain replayable with a NULL key for backward compatibility;
-- all new post commands require and persist a visible-ASCII idempotency key.
ALTER TABLE funding_events
    ADD COLUMN post_idempotency_key VARCHAR(255);

ALTER TABLE transfer_corrections
    ADD COLUMN post_idempotency_key VARCHAR(255);

ALTER TABLE funding_events
    ADD CONSTRAINT funding_post_idempotency_key_shape
    CHECK (
        post_idempotency_key IS NULL
        OR (
            length(post_idempotency_key) BETWEEN 16 AND 255
            AND post_idempotency_key ~ '^[!-~]+$'
        )
    );

ALTER TABLE transfer_corrections
    ADD CONSTRAINT correction_post_idempotency_key_shape
    CHECK (
        post_idempotency_key IS NULL
        OR (
            length(post_idempotency_key) BETWEEN 16 AND 255
            AND post_idempotency_key ~ '^[!-~]+$'
        )
    );
