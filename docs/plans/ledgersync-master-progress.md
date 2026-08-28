# LedgerSync master delivery register

**Authority:** [LedgerSync master completion plan](ledgersync-master-product-system-and-website-completion-plan.md)

**Established:** 2026-08-28

**Current milestone:** Local MVP requalification

**Current phase:** Phase 2 — make local startup deterministic and understandable

This register is the single repository status source for the master plan. Earlier plans and registers are retained as historical evidence. A phase is complete only when every exit criterion has evidence for the exact commit. Code, documentation, emulation, or a local test cannot substitute for a managed-provider result, physical-device review, legal decision, named owner, partner action, or production approval.

## Status vocabulary

| Status | Meaning |
|---|---|
| `COMPLETE` | Every phase exit criterion has commit-bound evidence. |
| `ACTIVE` | This is the current bounded implementation phase. |
| `PARTIAL` | Useful implementation exists, but one or more exit criteria remain unproven. |
| `PENDING` | Work has not started under this master plan. |
| `EXTERNAL_GATE` | Completion requires named human, partner, provider, credential, budget, device, or production authority. |
| `DEFERRED` | The product owner has deliberately kept the work outside the current milestone. |

## Master phase register

| ID | Phase | Status | Depends on | Accountable owner | Current evidence | Next stop-ship action |
|---|---|---|---|---|---|---|
| M00 | Canonical baseline | `COMPLETE` | None | Engineering/release | This plan, this register, preserved `1fa7709`, clean-tree baseline | Keep source, status, tasks, and evidence synchronized on every phase commit. |
| M01 | Current-main quality reconvergence | `COMPLETE` | M00 | Engineering/release | [Exact-commit Phase 1 quality evidence](../release-evidence/master-phase-1-quality.md) for `417bd0b`; responsive, CLS, ledger, browser, recovery, security, container, and real-stack gates passed | Requalify after any source, workflow, dependency, image, migration, contract, or supported-runtime change. |
| M02 | Deterministic local runtime | `ACTIVE` | M01 | Engineering/operations | `scripts/*-local.ps1`, local MVP evidence, loopback-only Compose | Audit every prerequisite/failure branch and prove start, diagnose, stop, backup, restore, and confirmed reset on the current commit. |
| M03 | Truthful dependency-aware UI | `PARTIAL` | M01 | Product/web | Operator state components, degraded-state tests, reconciliation evidence UI | Close remaining unavailable-versus-empty and prerequisite-gating gaps across every supported screen. |
| M04 | Guided first-run journey | `PARTIAL` | M02, M03 | Product/web | Existing onboarding guide and local-product Phase 8 evidence | Add server-owned resumable progress and connect the full funding-to-backup journey after M05 exists. |
| M05 | Controlled funding journals | `PENDING` | M01–M04 | Financial engineering + finance | Funding boundary is specified; no completed funding workflow is claimed | Approve accounting semantics, then implement test-first funding, approval, posting, reconciliation, UI, and compensation paths. |
| M06 | Compensation and approvals | `PENDING` | M05 | Financial engineering + finance/security | Existing immutable-transfer invariant only | Implement additive corrections, separation of duties, policy versioning, lifecycle safeguards, and step-up controls. |
| M07 | API-first developer product | `PARTIAL` | M05, M06 | Developer platform | Complete MVP OpenAPI and local developer view | Add credentials, webhooks, compatibility/deprecation policy, reviewed examples, and reproducible SDK workflow. |
| M08 | Operator-console IA | `PARTIAL` | M03, M05–M07 | Product/operations | Accounts, transfers, reconciliation, events, recovery, developer, local status | Add funding, approvals, webhooks, administration, and standardized evidence/filter/export behavior. |
| M09 | Unified design and accessibility | `PARTIAL` | M08 | Product design/accessibility | `DESIGN.md`, responsive tokens, automated browser/a11y/visual evidence | Extract shared primitives for both product surfaces and complete authorized physical-device/manual review. |
| M10 | Separate public website | `PENDING` | M07, M09 | Product/marketing/legal | Product positioning exists in repository docs | Build `site/` with evidence-backed content, pilot request, trust/legal/accessibility pages, consent, abuse controls, and monitoring. |
| M11 | Production identity and tenancy | `PARTIAL` | M05–M08 | Security/platform | Cognito validation, server-owned mappings, audited partner provisioning foundations | Add managed tenant/operator lifecycle and prove real Cognito MFA, PKCE, scopes, grants, revocation, and isolation. |
| M12 | Production AWS infrastructure | `PENDING` | M11 | Platform/security | AWS Mumbai target and threat model are documented | Implement reviewed Terraform; deployment remains gated by AWS account, budget, DNS, credentials, legal boundary, and named owners. |
| M13 | Observability and incident operations | `PARTIAL` | M12 | Operations/SRE | Local telemetry, alerts, dashboards, recovery runbooks | Bind SLOs, alerts, customer status, paging, ownership, and exercised runbooks to the managed environment. |
| M14 | Security, privacy, compliance, legal | `PARTIAL` | M10–M13 | Security/legal/product | Threat model and repository security gates | Complete policies and external reviews; no critical/high risk, legal conclusion, or penetration result may be self-certified. |
| M15 | Scale, resilience, backup, DR | `PARTIAL` | M12–M14 | Platform/operations/finance | Local 10,000-account, 25 TPS, fault, backup, restore, and reconciliation evidence | Prove managed load/fault behavior and provider-backed RDS PITR against the exact release candidate. |
| M16 | Design-partner pilot | `EXTERNAL_GATE` | M05–M15 | Product/partner/operations | Partner templates and graduation gates only | Contract and provision 2–3 approved partners, operate staged traffic, and close or accept findings with named owners. |
| M17 | Production release and operations | `EXTERNAL_GATE` | M16 | Product/release/operations/legal | Release workflow foundations only | Obtain go/no-go authority and execute staged production release with immutable traceability and ongoing reviews. |

## Previous-plan traceability

| Historical source | Classification under the master plan | Preserved evidence |
|---|---|---|
| `specs/001-secure-transfer-core/` | Implemented core; automated tasks complete except T094, T095, and T121 external/manual gates | Exact money, ledger, idempotency, RYEW, authorization, reconciliation, operator UI, and automated acceptance |
| `docs/plans/ledgersync-implementation-plan.md` | Historical implementation/audit plan; implemented items require current-commit requalification | Architecture remediation, responsive console, local and pilot evidence structure |
| `docs/plans/ledgersync-future-scope-implementation-plan.md` | Historical strategic roadmap; superseded where sequencing or scope differs | Long-range platform options and explicit deferrals |
| `docs/plans/ledgersync-roadmap-progress.md` | Historical status snapshot; superseded by this register | 2026-08-24 delivery state and external blockers |
| `docs/pilot/completion-gates.md` | Detailed historical/shared-pilot gate inventory; subordinate evidence input | Named gate IDs, owner categories, and non-fabrication rules |

Older documents must not be deleted. If their wording conflicts with the master plan or this register, the master plan and this register control.

## Stop-ship rules

- Never use floating-point values on a financial path or expose unsafe JSON numbers for money/version fields.
- Never edit or delete posted journals, audit evidence, approvals, or other immutable financial records.
- Never weaken authorization, idempotency, balance, reconciliation, RYEW, security, accessibility, performance, or recovery gates to obtain a pass.
- Never report unavailable evidence as an empty, matched, successful, or current business state.
- Never expose PostgreSQL, Redis, workers, migrations, seeds, diagnostics, or administration services publicly.
- Never deploy shared/provider infrastructure without approved account, budget, secret handling, legal boundary, and named operational ownership.
- Never claim physical-device, provider recovery, legal, penetration-test, partner, or production approval from repository automation.
- Any duplicate movement, unbalanced journal, tenant leak, negative customer balance, unexplained reconciliation mismatch, open exploitable critical/high finding, or missing recovery evidence blocks release.

## Phase evidence protocol

For every phase: preserve unrelated changes; execute test-first where financial contracts change; run targeted checks during development; run the complete phase gate before commit; inspect the diff and whitespace; scan staged content for secrets and generated noise; update this register and evidence; commit with `feat/fix/test/docs(scope) : outcome` plus truthful bullet points; push only to `origin`; wait for exact-commit CI; fix genuine failures; verify local/remote alignment; and leave the supported local runtime healthy.
