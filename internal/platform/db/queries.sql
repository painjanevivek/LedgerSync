-- name: GetOwnedAccount :one
SELECT a.id, a.tenant_id, a.currency, a.status, b.available_minor, b.ledger_minor, b.balance_version, b.updated_at
FROM accounts AS a
JOIN account_owners AS o ON o.account_id = a.id AND o.tenant_id = a.tenant_id
JOIN account_balance_projections AS b ON b.account_id = a.id
WHERE a.id = $1 AND a.tenant_id = $2 AND o.subject_id = $3;

-- name: LockAccountProjections :many
SELECT a.id, a.tenant_id, a.currency, a.status, b.available_minor, b.ledger_minor, b.balance_version
FROM accounts AS a
JOIN account_balance_projections AS b ON b.account_id = a.id
WHERE a.tenant_id = $1 AND a.id = ANY($2::uuid[])
ORDER BY a.id
FOR UPDATE OF a, b;

-- name: ReserveIdempotencyRequest :one
INSERT INTO idempotency_requests (
    tenant_id, actor_subject_id, operation, idempotency_key, request_fingerprint, state, expires_at
) VALUES ($1, $2, $3, $4, $5, 'in_progress', $6)
ON CONFLICT (tenant_id, actor_subject_id, operation, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetIdempotencyRequestForUpdate :one
SELECT *
FROM idempotency_requests
WHERE tenant_id = $1 AND actor_subject_id = $2 AND operation = $3 AND idempotency_key = $4
FOR UPDATE;

-- name: SubmitControlledTransfer :one
SELECT response_body, replayed
FROM controlled_submit_transfer_v1(
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: StoreIdempotencyOutcome :exec
UPDATE idempotency_requests
SET state = 'completed', response_status = $5, response_body = $6, transfer_id = $7, completed_at = $8
WHERE tenant_id = $1 AND actor_subject_id = $2 AND operation = $3 AND idempotency_key = $4
  AND state = 'in_progress';

-- name: EnqueueBalanceEvent :exec
INSERT INTO outbox_events (
    id, tenant_id, transfer_id, account_id, event_type, aggregate_version, payload, occurred_at
) VALUES ($1, $2, $3, $4, 'account.balance.changed.v1', $5, $6, $7);

-- name: ClaimOutboxEvents :many
WITH candidates AS (
    SELECT id
    FROM outbox_events
    WHERE published_at IS NULL AND available_at <= now()
    ORDER BY available_at, created_at
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
UPDATE outbox_events AS e
SET attempt_count = e.attempt_count + 1
FROM candidates
WHERE e.id = candidates.id
RETURNING e.*;

-- name: MarkOutboxPublished :exec
UPDATE outbox_events
SET published_at = $2, last_error_code = NULL
WHERE id = $1 AND published_at IS NULL;
