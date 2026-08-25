# Local-product Phase 10 — complete acceptance and recovery

**Result:** `PASSED`

**Verified:** 2026-08-25T02:07:41Z

**Gate:** [LPC-100](../pilot/local-product-completion-gates.md)

**Executable candidate:** `67d0b59a4265429e13b03194b289bcbbccba145d`. The resulting Phase 10 commit adds only this bounded evidence and its documentation links; it does not change the qualified executable tree.

**Boundary:** one Windows workstation, Docker Desktop, the loopback-only product at `http://127.0.0.1:3000`, one deterministic demo tenant, INR, internal LedgerSync accounts, and disposable isolated Compose projects. This is local-product evidence, not LAN, cloud, shared-host, production, bank-rail, custody, FX, or regulatory approval.

## Consolidated decision

The complete configurable local product passed one commit-bound automated journey. The harness preserved the ordinary `compose` PostgreSQL and Redis volumes, stopped only its containers while port 3000 was needed, built a uniquely named isolated stack from the candidate, exercised the full product and failure path, removed only exact validated acceptance resources, then rebuilt and health-checked the ordinary stack.

| Qualification | Result |
|---|---|
| Fresh isolated secrets, images, migrations, seed, API, worker, Redis, PostgreSQL, and web | `PASS` |
| Account create, same-key replay, changed-intent conflict, fund, freeze, denied write, reactivate, non-zero close denial, zero close, retained history | `PASS` |
| Transfer lost-response/same-key retry, one journal, balanced postings, immediate authorized balances and histories | `PASS` |
| Reconciliation same-key behavior, final matched state, zero mismatches | `PASS` |
| Financial result separated from outbox/delivery evidence | `PASS` |
| Transfer, account-ledger, and reconciliation CSV exactness and containment | `PASS` |
| Real Chromium journey, axe WCAG A/AA, keyboard behavior, and 320/390/640/768/1024/1366 CSS-pixel reflow | `PASS` |
| Redis, worker, API, web, and PostgreSQL restarts; repeatable migrations | `PASS` |
| Protected backup, digest validation, isolated restore, cache rebuild, matched reconciliation | `PASS` |
| Exact acceptance cleanup and ordinary project restoration | `PASS` |

The consolidated harness completed in **430.09 seconds** and reported `LOCAL_ACCEPTANCE=PASS`, `REAL_STACK_BROWSER=PASS`, `CAPACITY=PASS`, `PROTECTED_BACKUP_RESTORE=PASS`, `NORMAL_PROJECT_RESTORED=PASS`, and `ACCEPTANCE_CLEANUP=PASS`. Schema remained `000016_guided_read_models.up.sql`.

## Five-minute capacity evidence

The mixed workload used 25 paced virtual operators for exactly 300 iterations each, plus five one-minute control journeys. Pacing avoids k6 end-boundary over-counting, while exact PostgreSQL deltas prove that the requested work—not merely HTTP iterations—completed.

| Measure | Observed | Gate |
|---|---:|---:|
| Transfer iterations / durable transfers / journals / completed idempotency outcomes | **7,500 / 7,500 / 7,500 / 7,500** | exact equality |
| Ledger postings | **15,000** | exactly two per transfer |
| Low-rate account controls | **5** | exactly five |
| Achieved workload rate | **25.071 iterations/s** | 25 TPS profile |
| Transfer p95 | **212.397 ms** | < 500 ms |
| Authoritative balance p95 | **84.403 ms** | < 200 ms |
| PostgreSQL maximum connections | **6** | observed, bounded |
| Maximum sampled container CPU | **39.06%** | observed |
| Maximum sampled container memory | **2.42%** | observed |
| Deadlocks / dropped iterations / unexpected outcomes | **0 / 0 / 0** | all zero |
| Duplicate journals / tenant violations / negative projections / projection drift / velocity drift | **0 / 0 / 0 / 0 / 0** | all zero |
| Unpublished / dead outbox work after bounded drain | **0 / 0** | all zero |
| Post-load reconciliation | **matched, 0 mismatches** | required |

Metrics are local acceptance measurements under healthy dependencies. They are not production SLOs or external capacity promises.

## Recovery and preservation evidence

- The acceptance backup was streamed into its exact isolated state root, manifest/digest validated, restored into a uniquely named project, migrated, reconciled, and used to rebuild Redis from PostgreSQL.
- A separate protected normal-stack backup and isolated restore on the same executable candidate recorded a **22.38-second local RTO**, schema `000016_guided_read_models.up.sql`, matched reconciliation, zero invalid journals, zero invalid posted transfers, zero negative balances, and `NORMAL_PROJECT_UNCHANGED=PASS`.
- Restore and acceptance projects reported complete cleanup. No acceptance container or volume remained.
- The ordinary loopback stack was rebuilt from the qualified tree and finished healthy. Its opaque pre/post financial fingerprint, named volumes, migration, outbox, and reconciliation evidence matched; no raw balance or ledger dump is published here.

The RTO is a workstation observation, not a managed-provider or production recovery promise.

## Security and supply-chain evidence

The exact executable candidate passed all **20** recorded local qualification steps:

- full Go tests and `go vet`;
- **63.9%** critical financial-path coverage against the 60% floor;
- **108,097** exact-money fuzz executions over the required ten-second window;
- zero called vulnerabilities from pinned `govulncheck` and zero production npm audit vulnerabilities;
- 88-commit, 4.01 MB redacted history scan with no secrets;
- zero high/critical IaC findings across the three deployment Dockerfiles;
- exact-commit API, worker, and web image builds;
- SPDX JSON SBOM plus zero high/critical image-scan failures for each image;
- local image, Dockerfile, archive, SBOM, and vulnerability-report hashes recorded in ignored evidence state.

The local Go race detector was unavailable because local CGO race support is absent; the pinned Linux CI race job remains mandatory. Signed provenance is owned by `.github/workflows/security.yml`; the local record is intentionally unsigned.

## Defects found and closed before approval

1. Populated overview INR totals and the mobile operator action forced 320-pixel overflow. Component shrink boundaries, compact money typography, and navigation columns were corrected without hiding or clipping evidence.
2. The shared BFF private-read proxy collapsed duplicate filters. It now rejects duplicate, unknown, blank, and oversized query values before forwarding.
3. The capacity client used a transport hostname that correctly triggered the product's 421 DNS-rebinding defense. Docker host networking now reaches the exact configured loopback origin; host validation was not weakened.
4. Arrival-rate boundary ticks made exact durable row counts ambiguous. The driver now uses exact per-operator iterations paced to the required duration.
5. The mixed controls expected generic `items` arrays instead of the documented `events` and `transfers` envelopes. The checks now follow the real API contracts.
6. Security review findings covering actor/tenant binding, PostgreSQL role separation, Redis authority, digest-pinned fault/CI images, and deterministic Redocly execution were remediated and requalified.

## Deliberate limits

- Automated CSS viewports and Chromium accessibility checks are not physical-phone, physical-tablet, NVDA, VoiceOver, browser/OS-matrix, or production accessibility certification.
- PostgreSQL remains the sole financial authority. Redis publication/cache loss may reduce freshness or delivery visibility but cannot create, delete, or change ledger postings.
- No runtime secret, cookie, raw balance, CSV export, database dump, unbounded log, Playwright trace, or local backup is tracked as release evidence.
- Phase 11 may audit cleanup candidates, but Phase 12 cannot delete, extract, or split anything without explicit named user approval.
