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

The static JavaScript gate is enforced in CI. LCP, INP, and CLS remain measured release evidence because synthetic values depend on the agreed device/network profile; a missing trace is not treated as a pass.

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

## Capacity report template

| Run | TPS | p50 / p95 / p99 transfer ms | p95 balance ms | HTTP error % | DB CPU / connections | Redis memory | Reconciliation mismatch | Decision |
|---|---:|---|---:|---:|---|---|---:|---|
| Baseline | 10 | — | — | — | — | — | — | pending |
| Target | 25 | — | — | — | — | — | — | pending |
| Ceiling | 50 | — | — | — | — | — | — | pending |

An observed mismatch, duplicate movement, or unmet latency target blocks pilot
expansion until the cause is explained and a new run is recorded.
