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
4. Run `scripts/run-capacity-qualification.ps1`; it pins k6 by digest, samples
   the containers and PostgreSQL every five seconds, records Redis/outbox and
   financial invariants, and reconciles after the workload.
5. Archive the k6 JSON summary, trace/metric dashboard snapshot, and
   reconciliation result in the pilot evidence store.

The k6 scenario enters through the same-origin BFF and includes session/CSRF
establishment, one-minor-unit exact transfers, sampled same-key replay,
authoritative balance reads, account/history pages, and reconciliation reads.
Expected replay responses are measured separately from unexpected outcomes.

## 2026-08-24 Phase 1 capacity decision

The initial concentrated 50 TPS result was not accepted. A reproduction on the
accumulated demo database produced 10 exhausted serializable retries and 234
PostgreSQL rollbacks in 60 seconds. The transaction repeatedly scanned immutable
transfer history to calculate rolling limits while holding financial locks.

The remediation preserves serializable isolation and PostgreSQL authority. It
adds exact, rebuildable active-window velocity events/totals and sequences the
unavoidable tenant policy decision before starting the transaction. The Redis
worker also stopped issuing a benign `BUSYGROUP` command on every loop. See
[ADR-0012](architecture/adr-0012-bounded-transfer-capacity.md).

### Workload-shape diagnostics after remediation

Each 20-second shape traversed session/CSRF, transfers, balances, account and
history reads, plus reconciliation sampling. `retry` deliberately discarded a
response and replayed the same key. All unexpected-outcome counts were zero.

| Shape | TPS | Iterations | Transfer p50 / p95 / p99 | Balance p95 | Result |
|---|---:|---:|---:|---:|---|
| Hot pair | 10 / 25 / 50 | 200 / 501 / 1,001 | 14.22/17.04/27.64; 13.36/16.09/21.70; 13.35/49.45/68.51 ms | 158.37 / 157.63 / 157.75 ms | pass |
| Four-account mixed | 10 / 25 / 50 | 200 / 501 / 1,000 | 14.07/26.95/32.65; 13.87/18.54/30.03; 15.12/87.61/142.02 ms | 156.62 / 158.77 / 160.53 ms | pass |
| Retry-heavy | 10 / 25 / 50 | 201 / 500 / 1,001 | 13.29/42.47/58.00; 11.99/16.80/19.74; 12.16/50.63/97.36 ms | 141.69 / 157.65 / 157.48 ms | pass; 102 / 252 / 503 simulated lost responses |

### Five-minute qualification and saturation

The launch envelope is deliberately lower than the original aspirational range.
The partner limit is 25 new transfer journeys/second. PostgreSQL enforces 30
total write attempts/second and 1,800/minute across the tenant, reserving retry
capacity. The 50 TPS service run is the required 2× planning headroom evidence.

| Decision | Offered / achieved | Iterations | Transfer p50 / p95 / p99 | Balance p95 | Unexpected / dropped | Reconciliation |
|---|---:|---:|---:|---:|---:|---|
| **Pilot envelope — pass** | 25 / 24.993 TPS | 7,501 | 15.58 / 36.28 / 168.03 ms | 159.32 ms | 0 / 0 | matched, 0 (`dd9af0f0-0368-43c6-8818-87fb11466414`) |
| **2× service headroom — pass** | 50 / 49.988 TPS | 15,001 | 14.68 / 52.33 / 200.92 ms | 160.87 ms | 0 / 0 | matched, 0 (`1c1d7974-dd5f-49ab-93a0-53aaf3d594eb`) |
| **Saturation — not approved** | 60 / 59.475 TPS | 17,847 | 17.43 / 150.97 / 2,317.24 ms | 171.50 ms | 6 / 153 | matched, 0 (`b0466c44-c686-46d6-b33e-8d14528c6598`) |
| **Saturation — not approved** | 100 / 97.227 TPS | 29,320 | 54.62 / 2,595.58 / 4,664.33 ms | 1,376.07 ms | 147 / 680 | matched, 0 (`56a5d94b-c187-492f-817e-366c51acdbf3`, post-load) |

The 25 TPS controlled run recorded zero PostgreSQL rollbacks/deadlocks, maximum
six connections, maximum three active connections, and maximum two waiting
locks. Docker samples recorded average/maximum CPU of 16.23%/29.57% for the API,
20.34%/54.17% for PostgreSQL, 22.49%/62.83% for the web BFF, 3.52%/7.75% for
Redis, and 2.01%/3.95% for the worker. Redis returned zero errors; outbox
unpublished/dead counts were zero. Database growth was 47,513,600 bytes for this
synthetic write-heavy run.

These are local Docker Desktop measurements on an accumulated synthetic database,
not AWS sizing or a managed-environment SLO. CPU percentages are Docker per-core
figures and can exceed 100% in other runs. Provider capacity remains a later
managed-environment gate. Raising the partner limit requires a new commit-bound
qualification and explicit configuration change.

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
| Pilot | 25 | 15.58 / 36.28 / 168.03 | 159.32 | 0 | avg 20.34% / max 6 connections | 195,845,192-byte observed peak | 0 | pass locally; managed rerun remains |
| Headroom | 50 | 14.68 / 52.33 / 200.92 | 160.87 | 0 | avg 44.58% / max 6 connections | 40,695,296-byte observed peak at that run | 0 | pass as local 2× headroom |
| Saturation | 60 / 100 | not approved | not approved | above budget | captured | captured | 0 | do not raise pilot limit |

An observed mismatch, duplicate movement, or unmet latency target blocks pilot
expansion until the cause is explained and a new run is recorded.
