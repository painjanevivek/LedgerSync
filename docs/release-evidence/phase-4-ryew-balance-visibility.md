# Phase 4 — Read-your-writes balance visibility evidence

**Status:** Validated against isolated PostgreSQL 16 and Redis 7.4 on 2026-08-18.

## Delivered

- PostgreSQL outbox lease/ack/retry/dead state, Redis Streams publication, consumer-group pending recovery, and a monotonic version-aware cache.
- Signed, short-lived account/version consistency requirements that are retained in the HttpOnly BFF session and are never returned to browser code.
- An authorized private balance route that serves cache only when its version meets the requirement, otherwise waits briefly and reads PostgreSQL primary.
- Truthful `current_balance_unavailable` handling rather than presenting an old cache value as current.
- Outbox/cache/RYEW counters and a tenant-scoped `reconcile --rebuild-cache` command that repopulates Redis only from PostgreSQL projections.

## Live evidence

- `TestRequirementBearingReadFallsBackToPrimaryWhenProjectionIsDelayed` passed.
- `TestRedisLossUsesPrimaryAndRebuildsTheBalanceCache` passed.
- `TestBalanceProjectionIsIdempotentAndNeverRegressesVersion` passed.
- `go run ./cmd/reconcile --rebuild-cache --tenant-id 00000000-0000-0000-0000-000000000101` rebuilt two disposable cache records from PostgreSQL.
- `npm --prefix web run build` passed.

The proof environment used disposable containers only. PostgreSQL remains the authority; Redis was flushed during the test and was rebuilt from PostgreSQL without changing ledger, transfer, or balance-projection records.
