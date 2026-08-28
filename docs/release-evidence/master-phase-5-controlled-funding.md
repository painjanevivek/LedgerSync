# Master Phase 5 — controlled funding journals

**Qualified candidate:** `98ef5660c54cf72da9d37dba5bc349fbadc89f96`

**Captured:** 2026-08-28 (Asia/Calcutta)

**Decision:** Phase 5 repository exit gate passed; Phase 6 may begin

## Delivered financial control

- Added one hidden system funding-clearing account per tenant and currency, with negative balance authority restricted to that account kind.
- Added immutable, tenant-scoped funding events, append-only approval evidence, exact external references, policy versions, and rolling limit evidence.
- Added production dual control and an unmistakably server-owned local single-operator demo policy. Production funding remains fail-closed until finance activation is explicitly approved.
- Added exact-string money validation, account eligibility, authorization, idempotent request replay, per-command/operator/tenant limits, and stable pagination.
- Posts one balanced debit/credit journal, destination balance version, audit event, velocity record, and outbox evidence in a single serializable PostgreSQL transaction.
- Prevents direct customer balance editing, arbitrary opening balances, system-account selection, duplicate funding, partial compensation, and repeated compensation.
- Preserves the original event and journal while creating an additive compensation event with its own approval and balanced reversal.
- Reconciles the recorded customer-authorized external reference to exact debit and credit totals.
- Presents funding as “recorded external value evidence,” never as a bank deposit, settlement, or custody claim.

## Operator and API product

- Published scoped private routes for request, list, detail, approve, reject, post, compensate, and reconcile operations.
- Advanced the reviewed OpenAPI contract and safe developer metadata with fixed routes, exact money, retry outcomes, bounded errors, and `funding_event_id` lookup.
- Added a same-origin BFF with strict Host/origin/CSRF/scope checks, bounded JSON allowlists, timeouts, sanitized responses, and stable idempotency keys after unknown outcomes.
- Added a progressively disclosed evidence docket: external evidence, independent decision, balanced journal, on-demand reconciliation, and collapsed destructive compensation review.
- Hid system clearing accounts from ordinary customer account directories and transfer selectors.
- Connected the first-run journey to the exact posted funding record.

## Commit chain

| Commit | Purpose |
|---|---|
| `7bd1f98` | Define the controlled funding lifecycle and invariants. |
| `ee5a0e4` | Add serializable PostgreSQL funding, approval, posting, compensation, limits, and reconciliation. |
| `70ef888` | Remove secret-like test fixtures while retaining deterministic retry coverage. |
| `68004a0` | Publish the scoped private funding API and OpenAPI contract. |
| `abc1a47` | Add the BFF, evidence-docket UI, demo policy seed, and browser security boundary. |
| `97e9ae1` | Complete PostgreSQL-backed funding onboarding and real-stack lifecycle proof. |
| `2bd0bbd` | Correct the fresh-schema system-account category constraint. |
| `439c7a9` | Lock the fresh-upgrade, least-privilege journal path into regression coverage. |
| `1de94ca` | Make velocity-window timestamp inference deterministic on fresh PostgreSQL 16. |
| `2e26411` | Preserve upgrade fixtures, constrain navigation overflow, and promote reviewed Linux funding visuals. |
| `68478a5` | Narrow PostgreSQL row locks so the API retains read-only access to funding policy. |
| `6a842dc` | Add bounded loopback readiness diagnostics without trusting ambient proxies. |
| `3b7c26a` | Preserve the complete bounded operator scope set through the signed BFF/API boundary. |
| `ab5dbd4` | Align isolated-restore invariants with the authorized funding-clearing projection. |
| `98ef566` | Keep internal contra projections out of the customer-facing Redis rebuild. |

## Verification

| Gate | Result |
|---|---|
| Go application/platform/unit/contract | Passed locally, including funding domain, repository, HTTP handler, guidance, migration compatibility, vet, and race-compatible paths. |
| Live PostgreSQL integration | Passed locally: request/replay, production self-approval denial, independent approval, post/replay, equal postings, exact balance, outbox/audit, reconciliation, limits, compensation, and hidden system accounts. |
| Fresh upgrade regression | Passed locally by upgrading a temporary pre-funding database, using the least-privilege API role, creating the system account, and posting a balanced funding journal. |
| Web security/unit | Passed 90/90, including the bounded 17-scope operator session and 33-scope fail-closed case. |
| Web lint/build | Passed optimized Next.js production build with strict TypeScript. |
| OpenAPI | Passed Redocly validation and runtime drift coverage. |
| Production-path CI | Passed exact-candidate run `33166233627`. |
| Quality and isolated real stack | Passed exact-candidate run `33166233664`: Go formatting/vet/static analysis/race/fuzz/coverage, web test/lint/build/budgets, browser journeys and reviewed visual drift, disposable PostgreSQL/Redis integration, least-privilege upgrades, real BFF/API/worker funding lifecycle, replay after restart, seed idempotency, reconciliation, digest-bound backup and isolated restore, customer-only cache rebuild, dependency-fault recovery, and hardened container assertions. |
| Supply chain and security | Passed exact-candidate run `33166233616`: secret history, dependency audit, IaC scan, immutable container scans, SBOMs, and provenance attestations. |
| Contract validation | Passed run `33165575264` for `ab5dbd4`; `98ef566` changed only the internal cache projection query and its live integration regression, outside the contract workflow path filter. |

## Manual activation boundary

The implementation gate is complete, but no production tenant is activated by repository code. A named customer finance authority must approve debit/credit semantics, limits, policy version, and operator separation during provisioning. The `finance_activated` flag keeps production requests fail-closed until that manual decision is recorded; legal and custody-language confirmation remains a Phase 14 gate.

## Exit-gate decision

- A production-configured tenant can establish exact balances through a controlled journal without bank rails, custody claims, floating point, or manual SQL, once its explicit finance activation gate is satisfied.
- Same-key retries resolve to one durable funding event and one journal.
- Corrections are additive, exact, balanced, independently approved in production, and linked to the immutable original.
- Funding reconciliation proves the journal debit and credit against the customer-authorized external reference.
