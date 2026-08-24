# LedgerSync local-product completion gate register

**Status:** authoritative for work started after the passed local-only MVP

**Established:** 2026-08-24

**Input baseline:** `89f5752`

**Boundary:** one Windows workstation, Docker Desktop, web access only at `http://127.0.0.1:3000`, private Docker-only API/PostgreSQL/Redis services, INR demo data, internal same-currency ledger transfers, a server-controlled demo identity, and no external deployment

This register controls the next local-product completion cycle. It is additive to the [passed local-only MVP register](local-mvp-gates.md): it does not edit, revoke, reinterpret, or reuse the historical `L-010` through `L-070` results. Those results remain evidence for the release candidate at which they passed. Changes after baseline `89f5752` must satisfy the applicable `LPC-*` gates below before a new completion claim is made.

The shared production-pilot program remains governed separately by the [pilot completion register](completion-gates.md). Managed identity, cloud networking, provider PITR, legal approval, partner onboarding, and production traffic cannot be passed with this local evidence.

## Status rules

| Status | Meaning |
|---|---|
| `READY` | All predecessor inputs exist and the gate can be executed locally. |
| `IN_PROGRESS` | Work is underway and the exit criteria have not passed. |
| `BLOCKED_SEQUENCE` | A predecessor local gate has not passed. |
| `FAILED_REMEDIATE` | Objective evidence failed; completion remains blocked pending a fix and clean retest. |
| `PASSED` | Every exit criterion has current, commit-bound, reviewable evidence. |
| `OUT_OF_SCOPE_LOCAL_PRODUCT` | The requirement needs an external environment, authority, device, partner, or live traffic and remains governed by the production-pilot register. |

There is no partial or conditional pass. A duplicate movement, authorization disclosure, false-current balance, unexplained reconciliation mismatch, destructive recovery result, or unresolved critical/high issue blocks completion.

## Gates mapped to phases

The numeric suffix maps directly to the corresponding completion phase. The mapping preserves the `000` through `120` phase slots used by the repository's broader gate model while applying only the local boundary above.

| ID | Phase | Local-product outcome and exit criteria | Required evidence | Status | Dependency / next action |
|---|---|---|---|---|---|
| LPC-000 | Phase 0 — baseline, freeze, and stop-ship remediation | Baseline commit, immutable migrations, contracts, invariants, runtime state, preserved data, and limitations are recorded; the three known security stop-ships are fixed and independently retested without weakening the local boundary | [Phase 0 baseline evidence](../release-evidence/local-product-phase-0-baseline.md), focused security tests, full relevant Go/web regression results | `PASSED` | Security remediation commit `d674eaf`; verified `2026-08-24T17:52:53Z` |
| LPC-010 | Phase 1 — account domain and data | Zero-balance account creation, metadata validation, active/frozen/terminal-closed transitions, optimistic versioning, command idempotency, ownership, audit, and outbox persistence are atomic and cannot mutate financial projections directly | [Phase 1 account assurance](../release-evidence/local-product-phase-1-accounts.md), domain/property tests, forward-only migration evidence, PostgreSQL create/replay/conflict/close/concurrency tests, unchanged-account fingerprint, zero-mismatch reconciliation | `PASSED` | Verified on the Phase 1 working tree based on `0a730a9`; normal backup `backup-20260824T183310Z-0a730a9` passed |
| LPC-020 | Phase 2 — account command API and BFF | Additive account mutation routes enforce `accounts:write`, object authorization, strict bounded input, idempotency, optimistic concurrency, no-store, rate/capacity controls, CSRF, fixed origin/host, and response-unknown retry semantics | [Phase 2 account API/BFF assurance](../release-evidence/local-product-phase-2-account-api.md), OpenAPI/contract results, handler/BFF security tests, real BFF-to-PostgreSQL create/replay/lifecycle evidence, unchanged existing-route proof | `PASSED` | Verified on the coordinated Phase 2 working tree based on `8f560f3`; independently repeated suites and the supported-stack BFF lifecycle passed |
| LPC-030 | Phase 3 — configurable account UI | The operator can create an exact-zero INR account, fund it only through the existing balanced transfer, freeze/reactivate it, and permanently close it only at zero across supported viewport and accessibility states | [Phase 3 account UI assurance](../release-evidence/local-product-phase-3-account-ui.md), 44 unit/security tests, 64 mocked browser tests, 21 clean visual comparisons, and isolated real-stack lifecycle evidence | `PASSED` | Verified `2026-08-24T20:20:35Z` on the Phase 3 working tree based on `381ba05`; resulting commit binds evidence and implementation |
| LPC-040 | Phase 4 — reconciliation control center | An authorized operator can start at most one tenant reconciliation run, safely retry it, and inspect immutable matched/mismatch/already-running evidence without inferred success | Command/idempotency/concurrency tests, BFF security tests, matched and seeded-mismatch real-stack evidence, browser state/accessibility proof | `READY` | LPC-000 and LPC-030 passed; proceed with the operator-triggered reconciliation control center |
| LPC-050 | Phase 5 — diagnostics and event evidence | Bounded local health and sanitized event/delivery timelines distinguish PostgreSQL financial truth from Redis/cache and downstream delivery state without raw payload, host, Docker, or secret disclosure | Repository authorization/pagination tests, redaction corpus, dependency-partial tests, Redis/worker fault evidence, browser responsive/accessibility states | `BLOCKED_SEQUENCE` | Starts after LPC-000; integrates after LPC-040 |
| LPC-060 | Phase 6 — API-first developer workspace | Versioned OpenAPI and tested examples explain exact money, account/transfer/reconciliation/event contracts, safe retry, and errors; browser code cannot reveal credentials or send arbitrary requests | OpenAPI validation, example-schema tests, credential metadata/reveal/rotation/rollback script evidence, browser/security tests | `BLOCKED_SEQUENCE` | Starts after LPC-030 and requires contracts from LPC-040/LPC-050 |
| LPC-070 | Phase 7 — recovery center and exact exports | The UI safely presents bounded backup/restore evidence and host commands without execution privilege; authorized transfer/account/reconciliation CSV exports are streamed, exact, bounded, and spreadsheet-safe | Manifest containment/redaction tests, isolated restore result, CSV exactness/formula corpus/streaming/authorization tests, browser download evidence | `BLOCKED_SEQUENCE` | Starts after LPC-040 and LPC-050 |
| LPC-080 | Phase 8 — guided local experience | Direct dashboard entry remains; a dismissible orientation, stored-evidence explainability timeline, safe retry demonstration, and coherent Local tools navigation connect the product without fabricating financial facts | First-use/demo and empty-state journeys, partial timeline fixtures, same-key retry proof, navigation/context and responsive/keyboard results | `BLOCKED_SEQUENCE` | Starts after LPC-060 and LPC-070 |
| LPC-090 | Phase 9 — responsive, accessibility, performance, and design convergence | Every existing and additive screen follows `DESIGN.md`, uses truthful progressive states, remains operable at required viewports/input modes, and stays inside reviewed bundle/performance budgets | Full web unit/security/build/E2E/performance suite, axe/keyboard/zoom/reflow/forced-color results, reviewed snapshots and bundle report | `BLOCKED_SEQUENCE` | Starts after LPC-080 |
| LPC-100 | Phase 10 — complete local acceptance | One isolated journey proves account create/retry/conflict/fund/freeze/reactivate/close, transfer lost-response retry, reconciliation, event evidence, exports, dependency restarts, protected backup/restore, capacity, security, and preservation of normal data | Commit-bound acceptance transcript, financial fingerprints, fault/recovery/capacity metrics, scans, final normal-stack health and reconciliation | `BLOCKED_SEQUENCE` | Starts after LPC-090 |
| LPC-110 | Phase 11 — cleanup audit only | An evidence-backed, zero-deletion audit identifies unused/duplicated/oversized candidates with risk and confidence; every candidate below 90% is retained and temporary/user files are untouched | Review table with references, confidence, risk, recommendation, and required verification; clean working-tree comparison proving no source deletion | `BLOCKED_SEQUENCE` | Starts after LPC-100 and stops for named user approval |
| LPC-120 | Phase 12 — approved cleanup only | Only user-approved audit rows are deleted/extracted/split; behavior, dependencies, financial semantics, and public APIs remain unchanged | Approval-to-diff mapping plus proportional full tests, contract/runtime/browser evidence, and clean synchronized repository | `BLOCKED_SEQUENCE` | Starts only after explicit named approval; otherwise remains intentionally blocked without invalidating LPC-100 |

## Frozen boundary and change control

1. Migrations `000001` through `000012` are immutable. Any schema change starts at `000013`; editing, renumbering, or replacing an existing migration fails LPC-000.
2. The HTTP/OpenAPI and BFF actor-assertion contracts are frozen at baseline `89f5752`. A necessary correction must update runtime, contract, fixtures, tests, and evidence atomically and must not silently rename supported routes.
3. Exact integer minor-unit money, persistent idempotency, atomic balanced double entry, PostgreSQL financial authority, disposable Redis, account/object authorization, read-your-writes, same-currency movement, immutable posted history, compensating corrections, and truthful unknown/unavailable states are release invariants.
4. A code, migration, contract, Compose, security-boundary, workflow, or financial-state change reopens every affected downstream `LPC-*` gate.
5. Evidence must identify the candidate commit and UTC execution time. Secrets, cookies, tokens, database dumps, raw customer data, and unbounded logs must not be committed.

## Explicitly outside local-product completion

- public or non-loopback deployment;
- bank rails, cards, FX, custody, or external settlement;
- managed OIDC/SSO and real organization lifecycle;
- cloud infrastructure, public DNS, provider-managed secrets, and provider PITR;
- legal, regulatory, custody, contract, or production-finance approval;
- external device-farm sign-off, named on-call routes, design-partner credentials, customer data, or live traffic.

These items keep their status in the production-pilot register and cannot be converted to `PASSED` by an `LPC-*` result.

## Update protocol

1. Update a gate only with linked evidence, the exact candidate commit, and a UTC date.
2. Move an objective failure to `FAILED_REMEDIATE`; do not soften it to a conditional pass.
3. Keep later gates `BLOCKED_SEQUENCE` until their predecessor passes.
4. Do not modify the historical `L-*` register to describe this completion cycle.
5. Stop on any financial invariant failure, authorization disclosure, destructive recovery result, or unexplained mismatch.
