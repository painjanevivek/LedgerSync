# ADR-0012: Bound transfer-policy contention and the pilot write envelope

- **Status:** accepted for the controlled pilot
- **Date:** 2026-08-24
- **Scope:** same-currency internal ledger transfers only
- **Supersedes:** repeated rolling-window aggregation in the transfer transaction

## Context

The original 50 TPS hot-account evidence preserved correctness but exhausted
bounded serializable retries. Each transfer recalculated three rolling 24-hour
limits from the growing immutable transfer table while holding financial locks.
The first measured run recorded explicit retryable conflicts rather than false
success, but it exceeded the pilot error budget.

The pilot must retain serializable isolation, exact idempotency, double-entry
atomicity, and machine-enforced rolling limits. Redis cannot become a financial
coordinator. The launch envelope also needs to reserve capacity for safe
same-key retries after unknown outcomes.

## Options considered

1. Lower PostgreSQL isolation or move coordination into Redis. Rejected because
   either choice weakens the source-of-truth and failure semantics.
2. Add an application-instance mutex. Rejected because it does not coordinate
   multiple API replicas and fails during restart or rescheduling.
3. Keep scanning immutable transfers and increase retry counts. Rejected because
   it amplifies work and tail latency without removing the contention source.
4. Maintain exact, rebuildable 24-hour policy state in PostgreSQL and sequence
   the unavoidable tenant policy decision before opening each serializable
   transaction. Accepted as the least-risk pilot design.

## Decision

- PostgreSQL remains the only authority for transfer admission and posting.
- `transfer_velocity_events` stores the active exact 24-hour event window;
  `transfer_velocity_totals` stores tenant, actor, and source-account totals.
- Expired events are pruned and totals adjusted in the same transaction as the
  next policy decision. A posted transfer records its velocity event and all
  three totals atomically with ledger, projection, audit, outbox, and idempotent
  response state.
- A PostgreSQL session advisory lock sequences one tenant's required policy
  decision before `BEGIN ISOLATION LEVEL SERIALIZABLE`. It works across API
  replicas and avoids stale snapshots while queued. Serializable isolation is
  unchanged.
- The initial partner envelope is 25 new transfer journeys per second. The API
  enforces a tenant-wide ceiling of 30 total write attempts per second and
  1,800 per minute, leaving 20% for same-key recovery and scheduling jitter.
  Read requests are bounded at 6,000 per minute per tenant/principal/route.
- The capacity counters are shared PostgreSQL operational state. They are not
  financial evidence and may be removed by the approved retention job.

## Evidence and limits

The local Docker evidence demonstrates a five-minute representative 50 TPS
service run with zero unexpected outcomes and zero reconciliation mismatches.
This is 2× the 25 TPS partner envelope. Sustained 60 and 100 TPS runs exceeded
the local availability/latency envelope while preserving ledger invariants, so
neither rate is approved. Exact figures and environment limitations are recorded
in `docs/performance-baseline.md`.

This decision does not size AWS, approve a provider, or replace managed-environment
load and recovery evidence. The tenant-wide sequence is deliberately optimized
for the controlled pilot, not presented as an unbounded horizontal-scale design.

## Consequences

### Positive

- Rolling-limit work is bounded by the active window instead of total immutable
  history, and routine decisions no longer scan the transfer table.
- Multiple API replicas share one safe ordering point and one enforced admission
  envelope.
- Same-key retries remain safe and have explicit reserved capacity.
- Velocity state is explainable and rebuildable from posted transfers.

### Costs and follow-up

- Tenant policy decisions remain serialized; a single hot tenant cannot scale
  linearly by adding API replicas.
- The active-window table adds one row per posted transfer for up to 24 hours.
  Managed-environment sizing must measure storage, vacuum, index, and retention
  behavior using partner-informed traffic.
- Capacity-window counters use fixed windows, so partner SDKs must use bounded
  jitter and honor `429 Retry-After` with the original idempotency key.
- Raising the 25 TPS partner envelope requires a new measured result, explicit
  configuration change, reconciliation, and gate-register update.
