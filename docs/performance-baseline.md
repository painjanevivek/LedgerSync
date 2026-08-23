# Performance baseline and capacity report

This is the repeatable pilot measurement protocol, not a claim that a target
has already been achieved. Do not run it against production funds or a shared
pilot tenant without written approval and isolated load-test accounts.

## Targets

| Journey | Target | Measurement |
|---|---:|---|
| Exact internal transfer | p95 under 500 ms | HTTP response through the private API with healthy PostgreSQL and Redis |
| Authorized balance read | p95 under 200 ms | BFF or private API response, including cache/primary decision |
| Error rate | below 0.1% | Expected idempotent replays and intentional rejects reported separately |
| Financial mismatch | 0 | Reconciliation after every run |
| Largest compressed-independent route chunk | ≤350 KB raw JavaScript | `npm run test:performance` after `next build` |
| Total emitted route chunks | ≤2 MB raw JavaScript | `npm run test:performance` after `next build` |
| LCP | ≤2.5 s | throttled compact-device browser trace |
| INP | ≤200 ms | healthy-client interaction trace |
| CLS | ≤0.1 | browser performance trace across progressive loading |

The static JavaScript gate and the agreed compact-browser profile are enforced in
CI. The browser profile is Chromium at 390×844, 4× CPU slowdown, 75 ms latency,
4 Mbps download, and 1.5 Mbps upload. Its Playwright report contains the raw
LCP, INP, CLS, interaction count, and layout-shift sources. A missing metric or
report is not treated as a pass.

## Runbook

1. Create dedicated same-currency test accounts with enough prefunded balance.
2. Record database size, container CPU/memory limits, connection-pool settings,
   Redis memory, application revision, and test start time.
3. Supply short-lived test-only credentials through the environment. Never put
   credentials in the script or shell history.
4. Run `k6 run tests/performance/k6/transfers.js` at 10, 25, and 50 TPS for
   five minutes each, reconciling between runs.
5. Archive the k6 JSON summary, trace/metric dashboard snapshot, and
   reconciliation result in the pilot evidence store.

The k6 scenario enters through the same-origin BFF and includes session/CSRF
establishment, one-minor-unit exact transfers, sampled same-key replay,
authoritative balance reads, account/history pages, and reconciliation reads.
Expected replay responses are measured separately from unexpected outcomes.

## 2026-08-24 local diagnostic results

These 20-second runs used one demo operator and one source/destination pair on
Docker Desktop. They are useful contention diagnostics, not the required
five-minute production-like capacity qualification. The local run did not retain
p50/p99, host CPU/IO, pool occupancy, or lock-wait time-series, so those fields
remain explicitly unqualified.

| Run | Offered TPS | Iterations | Transfer p95 | Balance p95 | Unexpected outcomes | Result |
|---|---:|---:|---:|---:|---:|---|
| Baseline | 10 | 201 | 19.62 ms | 160.99 ms | 0 | PASS — 602/602 checks |
| Target | 25 | 501 | 19.04 ms | 157.10 ms | 0 | PASS — 1,410/1,410 checks |
| Hot-account ceiling | 50 | 1,000 | 240.62 ms | 166.43 ms | 26 | PAUSE — 26 retryable serializable conflicts (0.92% request failure) |

The 50 TPS result is a correct, explicit `503 transaction_conflict_retryable`
rather than an unexplained `500`, and retry instructions require the original
idempotency key. It still fails the below-0.1% error target. PostgreSQL recorded
520 transaction rollbacks and zero deadlocks across the diagnostic sequence.
Post-run reconciliation matched all six accounts with zero mismatches (run
`266205c8-0dee-4252-be98-ca601cbb386c`); unpublished and dead outbox counts were
both zero.

### Required remediation before capacity approval

1. Exercise a representative multi-tenant, multi-actor, multi-account workload;
   retain this single-pair case as the explicit hot-account ceiling scenario.
2. Replace repeated rolling-window scans under serializable isolation with an
   approved transactionally locked velocity-counter design, or demonstrate an
   equivalent bounded-contention control without weakening limits.
3. Retain bounded same-key client retry with jitter for the explicit retryable
   conflict response; never create a new key for an unknown outcome.
4. Run 10, 25, and 50 TPS for five minutes in the isolated pilot environment,
   capture p50/p95/p99, DB CPU/IO/connections/locks, pool saturation, Redis,
   outbox age, and reconcile between runs.
5. Demonstrate the agreed 2× planning headroom or record a signed scope/traffic
   reduction. Until then, partner traffic remains paused at this gate.

## Browser and asset evidence

- Production build: 10 JavaScript chunks, 659,957 bytes total, 229,156-byte
  largest chunk; zero font files. All configured budgets passed.
- The compact constrained-4G trace passed LCP ≤2.5 s, INP ≤200 ms, and CLS ≤0.1.
- A delayed bounded 100-row account history rendered progressively and navigation
  remained usable. Ledger rows use `content-visibility` with an intrinsic size to
  avoid rendering off-screen evidence eagerly without changing DOM semantics.

## Capacity report template

| Run | TPS | p50 / p95 / p99 transfer ms | p95 balance ms | HTTP error % | DB CPU / connections | Redis memory | Reconciliation mismatch | Decision |
|---|---:|---|---:|---:|---|---|---:|---|
| Baseline | 10 | Required 5-minute run | Required | Required | Required | Required | Required | pending production-like run |
| Target | 25 | Required 5-minute run | Required | Required | Required | Required | Required | pending production-like run |
| Ceiling | 50 | Required 5-minute run | Required | Required | Required | Required | Required | local hot-account run paused |

An observed mismatch, duplicate movement, or unmet latency target blocks pilot
expansion until the cause is explained and a new run is recorded.
