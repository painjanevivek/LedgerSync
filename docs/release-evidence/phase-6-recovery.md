# Phase 6 — Operational recovery evidence

**Date:** 2026-08-19
**Scope:** User Story 4 — recover and operate with confidence

## Delivered controls

- Immutable opening-balance baselines make ledger-to-projection reconciliation explicit for migrated accounts. A missing baseline is a mismatch, not a false match.
- A tenant-scoped `reconcile --run` persists `matched`/`mismatch` evidence and exits non-zero on mismatch; cache rebuild remains a separate PostgreSQL-to-Redis operation.
- OpenTelemetry OTLP/HTTP tracing and bounded metrics cover HTTP, transfer/reconciliation database work, cache, Redis Streams, worker loops, transfer outcomes, outbox age, and RYEW violations. Shared environments require a private collector endpoint.
- Prometheus alert rules and a Grafana operations dashboard cover RYEW violations, reconciliation mismatch, aged outbox work, dependency boundary errors, backup age, and restore result.
- Backup/PITR policy, isolated restore procedure, and database/Redis/outbox/RYEW/idempotency/secret-compromise runbooks are versioned. The provider policy is deliberately a template until the chosen managed PostgreSQL provider supplies real evidence.
- Manual release-evidence CI rejects backup age over 15 minutes, any failed restore drill, any RYEW violation, or a failed live migration/recovery suite.

## Isolated live validation

Disposable PostgreSQL 16 and Redis 7.4 were run only on `127.0.0.1:55432` and `127.0.0.1:56379`, then removed after the run.

| Check | Result |
|---|---|
| Migration application, repeatability, and preserved existing read contracts | PASS |
| Concurrent-transfer funds protection and idempotent replay/conflict | PASS |
| Redis cache loss, primary fallback, and monotonic version projection | PASS |
| Expired outbox lease recovery after worker loss | PASS |
| Redis publication failure reschedules without changing postings | PASS |
| Deliberately corrupted balance projection persists a reconciliation mismatch | PASS |
| Isolated PostgreSQL logical restore followed by tenant reconciliation | PASS — matched, 2 accounts, 0 mismatches |
| Go unit/contract/static analysis and web lint/BFF security tests | PASS |

The integration and fault packages were executed serially because their fixtures intentionally truncate a shared disposable database and Redis instance. The release workflow preserves that safe execution order.

## Remaining pilot gate outside source code

The local logical restore demonstrates the procedure but does not prove managed-provider continuous WAL archival. Before a shared pilot, select the managed PostgreSQL provider, configure its encrypted continuous WAL archival and isolated backup trust boundary, perform the first provider-backed isolated restore drill, and submit its measured backup age/RPO/RTO through the release-evidence workflow. Code cannot honestly claim those external controls are active without provider evidence.
