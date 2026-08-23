# LedgerSync Future-Scope Delivery Register

**Roadmap source:** `docs/plans/ledgersync-future-scope-implementation-plan.md`  
**Last updated:** 2026-08-24  
**Current delivery gate:** Phase 0 complete; Phase 1 is active and awaiting business-owned launch decisions.

This register is the evidence-backed companion to the future-scope implementation plan. It separates work that is demonstrably complete in the repository from work that depends on an approved business decision, third-party service, production environment, customer agreement, or measured operating result. A phase is never marked complete from code alone when its exit criteria require external evidence.

## Status at a glance

| Phase | Status | What the status means |
|---|---|---|
| 0 — Repository re-baseline and governance | **Complete** | The supported product boundary, deployment path, quality gates, and evidence sources are explicit and verified. |
| 1 — Pilot financial, identity, and security closure | **Active / decision-gated** | Repository controls are implemented; production identity, jurisdiction, pilot policy, partner, and recovery decisions remain open. |
| 2 — Managed platform, infrastructure, and recovery | **Pending** | Starts after the Phase 1 launch profile and target cloud/region are approved. |
| 3 — Controlled production pilot | **Pending** | Requires deployed infrastructure, approved controls, design partners, and operational ownership. |
| 4 — Monorepo product and public surfaces | **Pending** | Begins only after pilot evidence validates the product and operating model. |
| 5 — Developer platform and reliable webhooks | **Pending** | Requires stable pilot contracts and external-consumer requirements. |
| 6 — Ledger product expansion | **Pending** | New posting types remain subordinate to invariant and reconciliation proof. |
| 7 — Payment orchestration | **Conditional** | Activated only with a named regulated partner, jurisdiction, and compliance operating model. |
| 8 — AI safety foundation | **Pending** | Requires governed, permission-aware data and evaluation infrastructure. |
| 9 — AI copilots | **Pending** | Requires Phase 8 safety gates and remains advisory by default. |
| 10 — Measured scale and data platform | **Pending** | Triggered by observed capacity, latency, reliability, and analytics needs. |
| 11 — Geographic and enterprise expansion | **Conditional** | Requires jurisdiction-specific legal, compliance, residency, and operating approval. |

## Phase 0 completion record

### Outcome

LedgerSync now has one unambiguous supported product and deployment path. The repository presents the Go API, Next.js BFF/web application, PostgreSQL financial authority, Redis disposable cache, worker, migration tooling, and supporting contracts as the production-intent system. The previous root Compose topology is visibly isolated as an unsupported legacy reference, so a default local command cannot silently launch an obsolete architecture.

### Delivered controls

- The root Compose file delegates to the supported topology under `deploy/compose`.
- The legacy Compose file is named and labeled as historical and unsupported.
- Contract tests prevent production-intent files from depending on legacy `backend`, `dashboard`, or `simulation` paths.
- The OpenAPI document describes every runtime route, security boundary, exact-money string, and known transfer-outcome behavior.
- Database roles enforce least privilege for migrations, the API, and the delivery worker while preserving the runtime reads each role actually needs.
- Append-only financial and audit tables are protected at the database boundary.
- Shared PostgreSQL rate limits cover tenant, principal, and route dimensions; money-moving writes fail closed if the limiter is unavailable.
- Pilot policies enforce currency, positive amount, per-transfer limit, actor rolling limit, tenant rolling limit, and destination-credit authorization inside trusted server-side boundaries.
- Timeout handling distinguishes an unknown transfer outcome from an ordinary read timeout and preserves safe same-key idempotent retry guidance.
- Audit denials retain useful actor, tenant, route, and decision metadata without leaking raw balances or transfer amounts.
- Browser financial semantics distinguish unavailable, pending, stale, and known-zero values instead of inventing certainty.

### Verification evidence

The following gates passed on 2026-08-24:

- Go command, application, unit, and contract tests.
- PostgreSQL integration tests, including migration compatibility, least-privilege roles, pilot controls, rate limiting, and ledger invariants.
- Fault-injection tests for degraded PostgreSQL, Redis, worker, and network behavior.
- Web linting, 17 unit/security tests, and the optimized production build.
- 15 browser end-to-end checks covering accessibility, compact navigation, forced colors, reduced motion, zoom/reflow, exact money, idempotent retry, timeout semantics, and authorization-safe states.
- OpenAPI linting with Redocly and no findings.
- JavaScript performance budgets for total and largest chunk size.
- Canonical root and supported Compose configuration validation.
- The secure-transfer requirements checklist remains 16/16 complete.

### Phase 0 commit scope

The phase is intended to be committed with this copy-pastable message:

```text
feat(pilot-security) : complete the Phase 0 delivery baseline

- enforce pilot currency, amount, rolling velocity, destination authorization, and shared route limits inside trusted boundaries
- align runtime, OpenAPI, timeout, evidence, database-role, and lossless financial contracts
- isolate the legacy demo topology and prove Go, database, fault, browser, accessibility, and performance gates
```

## Phase 1 active gate

### Repository work already delivered

The technical pilot-control slice is present: exact money, atomic ledger posting, idempotency, safe reads, tenant isolation, destination authorization, rolling velocity controls, shared route limiting, least-privilege roles, audit evidence, contract parity, and truthful UI timeout/degraded states.

### Decisions and external evidence still required

The following items must be explicitly approved before Phase 1 can be declared complete:

1. **Pilot buyer and daily user:** name the initial segment and design partners so workflows, permissions, service levels, and evidence match a real operating team.
2. **Launch surface:** confirm whether the API or the operator web application is the primary launch surface, while retaining both where required.
3. **Money-movement boundary:** confirm that launch is internal ledger transfer only, or name the regulated provider and rails if external settlement is included.
4. **Legal and compliance profile:** approve the launch jurisdiction, contracting entity, data-residency obligations, retention periods, sanctions/KYC/KYB responsibilities, and incident-notification duties.
5. **Financial pilot policy:** approve the initial currency, per-transfer minimum and maximum, actor rolling limit, tenant rolling limit, timezone/window semantics, and exception owner.
6. **Identity provider:** select the production OIDC provider, define issuer/audience/client configuration, map tenant and role claims, require MFA where appropriate, and document joiner/mover/leaver handling.
7. **Target platform:** select cloud, region, network boundary, secrets/KMS service, managed PostgreSQL/Redis products, and observability destinations.
8. **Recovery objectives:** approve RPO, RTO, backup retention, restore-test frequency, incident severity levels, and accountable on-call owners.
9. **Pilot success criteria:** set measurable targets for customers, accounts, transfers, error rate, duplicate-posting rate, reconciliation mismatches, support response, latency, and pilot duration.

### Why work does not jump directly to Phase 2

Infrastructure, identity, network, backup, and compliance implementations encode the decisions above. Building them before those decisions would create expensive rework and could produce controls that are technically polished but legally or operationally wrong. Repository-safe preparation may continue, but Phase 1 remains open until the launch profile is approved and its evidence is captured.

## Evidence rules for every later phase

- A repository change is complete only when its focused tests and affected end-to-end path pass.
- A security control is complete only when its enforcement point, denial behavior, audit evidence, and failure mode are tested.
- A database change is complete only when forward migration, compatibility, privileges, integrity invariants, backup implications, and rollback/forward-fix strategy are documented.
- A production phase is complete only when deployment, monitoring, recovery, incident ownership, and customer-facing behavior have objective evidence.
- An AI phase is complete only when permissions, prompt-injection resistance, data provenance, evaluation thresholds, human override, auditability, cost limits, and kill switches are demonstrated.
- External payment movement and geographic expansion remain conditional; neither is inferred from an internal-ledger implementation.

## Change-control protocol

For each phase, update this register with the delivered scope, test evidence, unresolved risks, external approvals, commit SHA, and deployment evidence. Then create one focused phase commit using the agreed `feat/fix(<scope>) : <summary>` subject style and push it only after the phase gate is satisfied. If a phase is only partially complete, record it as active and do not present it as done.
