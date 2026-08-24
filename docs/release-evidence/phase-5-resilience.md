# Phase 5 — Financial safety, fault, capacity, and local recovery evidence

Evidence date: 2026-08-24 Asia/Calcutta (2026-08-23 UTC runtime logs)

## Decision

Transfer correctness, dependency-fault behavior, compact browser budgets, local
isolated restore, and the conservative local capacity envelope are qualified.
The earlier concentrated 50 TPS conflict was remediated without weakening
serializable isolation. The initial partner limit is now 25 TPS, protected by a
tenant-wide 30-attempt/second and 1,800-attempt/minute admission envelope. A
five-minute 50 TPS run passed as 2× service headroom. Managed-provider capacity
evidence remains separate and incomplete.

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

## Local workload evidence after remediation

| Offered TPS | Duration | Iterations | Transfer p95 | Balance p95 | Unexpected / dropped | Decision |
|---:|---:|---:|---:|---:|---:|---|
| 25, enforced mixed | 5 min | 7,501 | 36.28 ms | 159.32 ms | 0 / 0 | pilot envelope pass |
| 50, four-account mixed | 5 min | 15,001 | 52.33 ms | 160.87 ms | 0 / 0 | 2× service headroom pass |
| 60, four-account mixed | 5 min | 17,847 | 150.97 ms | 171.50 ms | 6 / 153 | saturation; not approved |
| 100, four-account mixed | 5 min | 29,320 | 2,595.58 ms | 1,376.07 ms | 147 / 680 | saturation; not approved |

The workload traversed the real BFF session and CSRF boundary. It mixed exact
transfers, distributed same-key retries, authoritative balances, accounts,
history, and reconciliation reads. Every run reconciled with zero mismatches;
the accepted 25 TPS run is `dd9af0f0-0368-43c6-8818-87fb11466414`. Unpublished
and dead outbox counts, duplicate journal movements, negative projections,
tenant violations, and velocity drift were all zero. See
`docs/performance-baseline.md` for percentiles, resource signals, saturation,
and local-environment limitations.

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

`scripts/local-restore-drill.ps1 -ComposeProject compose` restored an
80,011,017-byte logical dump into a
fresh internal PostgreSQL 16 container, ran the migration binary, rebuilt a new
Redis 7.4 instance, and reconciled the restored tenant.

| Check | Result |
|---|---|
| Applied schema migrations | 12 |
| Restored accounts | 6 |
| Restored posted transfers | 140,582 |
| Reconciliation | `matched`, 0 mismatches |
| Reconciliation run | `16c65ee4-0cb1-4f91-84e0-3fda4a66748a` |
| Rebuilt Redis keys | 6 |
| Local procedure time | 27.83 seconds |
| Isolated-resource cleanup | complete |

This proves logical restore compatibility and cache reconstruction, not managed
PITR or a production RPO/RTO. Provider-backed recovery remains a Phase 7 gate.

## Release gate

The local capacity pause is closed with an enforced lower partner envelope and
2× service headroom. This does not authorize external traffic: physical-device,
finance/security/legal, managed identity/infrastructure, provider PITR,
operational-tabletop, and design-partner gates remain open.
