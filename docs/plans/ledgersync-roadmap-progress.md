# LedgerSync Future-Scope Delivery Register

**Roadmap source:** `docs/plans/ledgersync-future-scope-implementation-plan.md`  
**Last updated:** 2026-08-24  
**Current delivery gate:** Phase 0 complete; Phase 1 repository closure is implemented and awaiting managed AWS/Cognito, recovery, partner, and accountable-human evidence.

This register is the evidence-backed companion to the future-scope implementation plan. It separates work that is demonstrably complete in the repository from work that depends on an approved business decision, third-party service, production environment, customer agreement, or measured operating result. A phase is never marked complete from code alone when its exit criteria require external evidence.

## Status at a glance

| Phase | Status | What the status means |
|---|---|---|
| 0 — Repository re-baseline and governance | **Complete** | The supported product boundary, deployment path, quality gates, and evidence sources are explicit and verified. |
| 1 — Pilot financial, identity, and security closure | **Active / environment-gated** | Launch decisions and repository controls are complete; the real AWS/Cognito topology, provider restore, named partner, and approvals remain unproven. |
| 2 — Managed platform, infrastructure, and recovery | **Blocked external / technically ready** | AWS Mumbai/Cognito is the selected technical baseline, but account, budget, legal boundary, credentials, DNS, and accountable owners are not authorized. |
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
- Web linting, 20 unit/security/semantics tests, and the optimized production build.
- 45 browser end-to-end checks covering accessibility, six responsive viewports, compact navigation, forced colors, reduced motion, zoom/reflow, exact money, idempotent retry, timeout semantics, progressive history, and 19 reviewed visual states.
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

### Approved decisions

The approved profile is an India-only, non-custodial, internal-ledger product for vertical-SaaS and fintech-infrastructure teams. The API is the primary integration surface and the invite-only operator console is required from day one. INR is the sole pilot currency. AWS Mumbai and Cognito are selected, with encrypted backup copies in Hyderabad, explicit recovery objectives, capacity targets, compliance posture, and graduation metrics. See `docs/pilot/india-launch-profile.md`.

### Repository work delivered

The technical pilot-control slice includes exact money, atomic ledger posting, idempotency, safe reads, tenant isolation, destination authorization, approved INR amount/rolling limits, shared route limiting, least-privilege roles, audit evidence, contract parity, and truthful UI timeout/degraded states.

- Cognito API access tokens require `token_use=access`, `client_id`, the configured LedgerSync resource audience, and allowlisted scopes.
- Partner/BFF app-client IDs map to tenants on the server; a token claim or request parameter cannot choose tenant authority.
- Operator OIDC subjects map to tenant, roles, and scopes in invite-only server configuration; Cognito ID tokens require `token_use=id`.
- A BFF workload token carrying `bff:act-as-user` is rejected unless it includes a valid, unreplayed, short-lived actor assertion.
- Supported configuration, demo/provisioning data, OpenAPI examples, browser evidence, and performance traffic use INR paise and the approved pilot limits.
- `docs/security/LedgerSync-threat-model.md` models the validated AWS/Cognito topology, residual risks, abuse paths, and required detections.

### External evidence still required

Phase 1 cannot be declared complete until the following evidence exists:

1. The actual Cognito pool/app clients disable self-registration, require MFA for operators, bind access tokens to the LedgerSync resource, and pass the real-token allow/deny matrix.
2. AWS edge, WAF, private subnets, security groups, managed secrets, PostgreSQL/Redis encryption, workload/database roles, and India-region observability are deployed and independently reviewed.
3. A provider-backed isolated PITR restore meets the approved RPO/RTO, Redis is rebuilt, reconciliation reports zero differences, and write reopening is approved.
4. Named product, finance, security, operations, legal/compliance, incident, and partner owners approve the release candidate.
5. At least one contracted design partner, account set, support path, IP policy, and observation window are approved.
6. A clean checkout passes all release suites in the target environment with no unresolved critical/high security finding.

### Why work does not jump directly to Phase 2

The technical decisions required to design Phase 2 now exist, so Codex can prepare reviewed infrastructure code and cost/exit material. Actual resource creation remains blocked until account/budget authorization, the legal boundary, approved secret handling, and time-bounded credentials exist. Phase 1 remains open in parallel as an environment evidence gate: infrastructure code, a local test, or a document cannot substitute for real Cognito tokens, deployed network reachability, provider restore results, counsel review, or accountable sign-off.

## Evidence rules for every later phase

- A repository change is complete only when its focused tests and affected end-to-end path pass.
- A security control is complete only when its enforcement point, denial behavior, audit evidence, and failure mode are tested.
- A database change is complete only when forward migration, compatibility, privileges, integrity invariants, backup implications, and rollback/forward-fix strategy are documented.
- A production phase is complete only when deployment, monitoring, recovery, incident ownership, and customer-facing behavior have objective evidence.
- An AI phase is complete only when permissions, prompt-injection resistance, data provenance, evaluation thresholds, human override, auditability, cost limits, and kill switches are demonstrated.
- External payment movement and geographic expansion remain conditional; neither is inferred from an internal-ledger implementation.

## Change-control protocol

For each phase, update this register with the delivered scope, test evidence, unresolved risks, external approvals, commit SHA, and deployment evidence. Then create one focused phase commit using the agreed `feat/fix(<scope>) : <summary>` subject style and push it only after the phase gate is satisfied. If a phase is only partially complete, record it as active and do not present it as done.
