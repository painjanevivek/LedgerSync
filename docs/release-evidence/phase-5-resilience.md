# Phase 5 — Financial safety, fault, capacity, and local recovery evidence

Evidence date: 2026-08-24 Asia/Calcutta (2026-08-23 UTC runtime logs)

## Decision

Transfer correctness, dependency-fault behavior, compact browser budgets, and a
local isolated restore are qualified. The final production-like capacity gate is
**paused**, because a deliberately concentrated 50 TPS workload exceeded the
0.1% error budget with explicit retryable PostgreSQL serialization conflicts.
No correctness or reconciliation failure was observed.

## Safety and fault evidence

| Evidence | Result |
|---|---|
| Exact-money fuzzing | PASS — 211,041 executions in approximately 12 seconds; 17 new interesting inputs |
| Live PostgreSQL/Redis integration suite | PASS in 11.766 seconds, including 10,000 accounts, no-overdraft concurrency, idempotency, authorization, retention, replay, and reconciliation |
| Controlled dependency-fault suite | PASS — all 5 cases in 5.732 seconds |
| Worker lease recovery | PASS — expired work reclaimed after interruption |
| Redis publication failure | PASS — delivery rescheduled without changing committed postings |
| Go race detector | Not run locally: Windows CGO toolchain unavailable; mandatory Ubuntu `go test -race` CI gate remains configured |

The API now returns `503 transaction_conflict_retryable` after bounded
serializable retry exhaustion. The response tells clients to retry the original
request with the same idempotency key, preserving unknown-outcome safety.

## Local workload evidence

| Offered TPS | Duration | Iterations | Transfer p95 | Balance p95 | Unexpected | Decision |
|---:|---:|---:|---:|---:|---:|---|
| 10 | 20 s | 201 | 19.62 ms | 160.99 ms | 0 | diagnostic pass |
| 25 | 20 s | 501 | 19.04 ms | 157.10 ms | 0 | diagnostic pass |
| 50, one hot account pair | 20 s | 1,000 | 240.62 ms | 166.43 ms | 26 retryable conflicts | pause/remediate |

The workload traversed the real BFF session and CSRF boundary. It mixed exact
transfers, sampled same-key retries, authoritative balances, accounts, history,
and reconciliation reads. Post-run reconciliation matched six accounts with
zero mismatches (`266205c8-0dee-4252-be98-ca601cbb386c`), and both unpublished
and dead outbox counts were zero. See `docs/performance-baseline.md` for the
limitations and required remediation.

## Browser evidence

- Production bundle: 659,957 bytes across 10 JavaScript chunks; largest 229,156
  bytes; no font assets; all static budgets passed.
- Compact test profile: Chromium 390×844, 4× CPU slowdown, 75 ms latency,
  4 Mbps down, 1.5 Mbps up.
- LCP, INP, and CLS budgets passed after rendering the same full console shell
  during session resolution and reserving stable document/row geometry.
- A delayed 100-entry ledger history progressively rendered and navigation
  remained responsive.

CI uploads the Playwright report and raw attached web-vital JSON for review.

## Isolated local restore evidence

`scripts/local-restore-drill.ps1` restored a 1,537,059-byte logical dump into a
fresh internal PostgreSQL 16 container, ran the migration binary, rebuilt a new
Redis 7.4 instance, and reconciled the restored tenant.

| Check | Result |
|---|---|
| Applied schema migrations | 11 |
| Restored accounts | 6 |
| Restored posted transfers | 2,746 |
| Reconciliation | `matched`, 0 mismatches |
| Reconciliation run | `61aa1c01-69d9-4adb-b60f-5dcb5ab6caea` |
| Rebuilt Redis keys | 6 |
| Local procedure time | 4.93 seconds |
| Isolated-resource cleanup | complete |

This proves logical restore compatibility and cache reconstruction, not managed
PITR or a production RPO/RTO. Provider-backed recovery remains a Phase 7 gate.

## Release gate

Phase 5 satisfies its acceptance rule through an explicit pause/remediation
decision: outcomes remained exact and reconcilable, but production-like capacity
and 2× headroom are not approved. External partner traffic must not begin until
the full five-minute representative workload passes with saturation telemetry.
