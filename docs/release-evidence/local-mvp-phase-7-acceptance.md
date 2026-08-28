# Local MVP Phase 7 — consolidated acceptance evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows workstation and Docker Desktop

**Tested source commit:** `a08cd4ded92dae12de134459d1747167693972e0`

**Evidence binding:** the Git commit containing this document

## Release decision

LedgerSync is ready to use as a local-only MVP on one Windows workstation at `http://127.0.0.1:3000`. The acceptance harness started a uniquely named clean-room stack, exercised the complete operator journey, qualified the target load, proved recovery, removed only its isolated resources, and restored the persistent `compose` stack without changing its existing ledger state.

This result does not claim a shared production pilot, public deployment, custody, bank connectivity, managed identity, provider-backed recovery, or physical-device sign-off.

## Clean-room journey

| Acceptance step | Observed result |
|---|---|
| Preflight | Clean tracked `main`, Docker available with the required CPU/memory floor, and no project/state collision |
| Runtime | Seven-service isolated stack healthy; all 12 migrations applied through `000012_transfer_velocity_capacity.up.sql` |
| Exact transfers | Three INR transfers posted through the real BFF: 123, 234, and 345 minor units |
| Idempotent retry | Replaying the identical request with the same key returned the original transfer and caused no second balance movement |
| Immediate visibility | Source and authorized destination balances satisfied their signed read-your-writes versions immediately after each posted transfer |
| Explainability | Transfer detail, account history, journal, and two balanced immutable postings agreed |
| Dependency recovery | Redis, worker, API, web, and PostgreSQL were restarted; migration rerun remained idempotent |
| Reconciliation | `matched`; zero mismatches |
| Backup and restore | PostgreSQL dump streamed to protected host storage, validated, restored into an isolated stack, and disposable Redis state rebuilt |
| Cleanup | Acceptance containers, networks, volumes, and state directory removed; normal stack and original state fingerprint restored |

The three acceptance transfer identifiers were `5591b115-ccfb-4a87-9157-dfe04a7a86da`, `f3aa8cc6-d176-475f-8e67-f63f3ac043a0`, and `2d894552-f65a-4675-b357-73f1054405da`. They identify disposable test data only; the isolated database was deleted after the pass.

## Five-minute capacity qualification

| Measure | Observed result | Gate |
|---|---:|---:|
| Offered rate | 25 TPS for 5 minutes | 25 TPS |
| Completed iterations | 7,500 | 7,500 expected |
| Achieved rate | 24.9918 iterations/s | No material shortfall |
| Dropped iterations | 0 | 0 |
| Unexpected outcomes | 0 | 0 |
| HTTP failure rate | 0 | 0 |
| Transfer latency p50 / p95 / p99 / max | 14.57 / 21.27 / 30.52 / 94.11 ms | p95 < 500 ms |
| Balance latency p95 / p99 / max | 158.33 / 160.80 / 174.28 ms | p95 < 200 ms |
| PostgreSQL rollbacks / deadlocks / waiting locks | 0 / 0 / 0 | 0 safety-impacting events |
| Redis error replies | 0 | 0 |
| Unpublished / dead outbox records | 0 / 0 | 0 / 0 |
| Duplicate journal movements | 0 | 0 |
| Tenant-boundary violations | 0 | 0 |
| Negative balance projections | 0 | 0 |
| Velocity-counter mismatches | 0 | 0 |
| Reconciliation | `matched`, 0 mismatches | `matched`, 0 mismatches |

The capacity reconciliation run was `a0516d6e-4f50-4269-bac4-84e6b1e7f4f1`. Peak measured container CPU was 88.11% for the web BFF and 47.60% for PostgreSQL; peak container memory percentages remained below 4%. These are local Docker Desktop measurements, not production forecasts.

## Proportional release matrix

| Check | Observed result |
|---|---|
| Go formatting and vet | Passed |
| Full Go suite | Passed: internal, contract, fault, integration, system, and unit packages |
| Money parser fuzzing | Passed for 10 seconds; 220,098 executions and no failure |
| Critical Go coverage | 64.8%; 60% gate passed |
| Go vulnerability scan | 0 called vulnerabilities |
| Web lint | Passed |
| Web unit/security suite | 25 passed |
| Next.js production build | Passed |
| Production npm audit | 0 vulnerabilities |
| JavaScript/font budgets | Passed; 663,477 total JS bytes, 229,156 largest chunk, no webfonts |
| Functional/responsive/accessibility/state/visual browser suite | 48 passed |
| Browser performance suite | 2 passed; LCP 376 ms, interaction 24 ms, CLS 0.0777 |
| Live hardened-boundary script | Passed: generated secrets, seven hardened containers, loopback-only publication, redacted logs, and authenticated reads |
| Final application-image scan | API, worker, and web each reported 0 critical, 0 high, 0 medium, and 0 low vulnerabilities |
| Tracked-source secret signatures | No private-key, cloud-key, live-token, or former fixed-development credential signature found |

The Go race detector is retained as a Linux CI gate because the local Windows environment has CGO disabled and no C compiler. No local race result is claimed.

## Supported boundary and remaining external gates

The accepted product boundary is local Docker Desktop, loopback-only browser access, deterministic INR demo data, and internal same-currency ledger transfers. Responsive behavior was automated at desktop, tablet, and mobile viewports; no physical-device validation is claimed.

Managed OIDC/SSO, public or cloud infrastructure, provider PITR, external alert destinations, legal/custody decisions, real partner credentials, physical-device review, and production traffic remain `OUT_OF_SCOPE_LOCAL_MVP`. They continue to be tracked by the production-pilot register and do not weaken this local release result.
