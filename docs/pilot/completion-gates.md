# LedgerSync pilot completion gate register

**Status:** historical detailed pilot-gate inventory; subordinate to the [master delivery register](../plans/ledgersync-master-progress.md)

**Established:** 2026-08-24

**Input baseline:** `22f9fdc0ff329be2e11f845d49cf1174e5fad913`

**Scope:** engineering MVP → controlled design-partner pilot → evidence-based graduation

This register preserves the detailed shared-pilot gates established on 2026-08-24. Current cross-phase status is maintained in the master delivery register. A repository test never substitutes for a real device, managed provider, legal decision, named operator, partner consent, or operating history.

## Status rules

| Status | Meaning |
|---|---|
| `READY` | Inputs exist and Codex can start the next work without inventing evidence. |
| `IN_PROGRESS` | Work is actively executing and has not yet met its exit criteria. |
| `BLOCKED_EXTERNAL` | A credential, provider, physical device, named human decision, or partner action is required. |
| `FAILED_REMEDIATE` | Objective evidence failed; movement/expansion stays paused until a fix and retest pass. |
| `PASSED` | Every stated exit criterion has reviewable evidence. |
| `NOT_APPLICABLE` | An accountable decision records why the gate does not apply. |
| `OUT_OF_SCOPE_LOCAL_MVP` | The gate remains valid for a shared production pilot but cannot block the one-workstation, loopback-only demo release. See the [local MVP register](local-mvp-gates.md). |

“Mostly done,” “conditional pass,” and undocumented verbal approval are prohibited.

This register remains authoritative for a shared design-partner pilot. The separate [local-only MVP gate register](local-mvp-gates.md) reclassifies external requirements only inside the one-workstation boundary and does not modify any historical status or evidence below.

## Authoritative gates

| ID | Plan/task | Gate and plain-language outcome | Accountable owner | Evidence | Status | Expiry/review | Blocker class | Next action |
|---|---|---|---|---|---|---|---|---|
| G-000 | Phase 0, TASK-001, T096 | Repository, task, test, screenshot, and CI claims agree | Engineering/release owner | [operator UI](../release-evidence/operator-ui.md), [Spec Kit tasks](../../specs/001-secure-transfer-core/tasks.md), [roadmap register](../plans/ledgersync-roadmap-progress.md), commit-bound Quality artifact | `PASSED` | Every source or workflow change | Internal | Keep all required workflows green for the commit containing this register |
| G-010 | Phase 1, TASK-012 | Enforced 25 TPS partner envelope and measured 2× service headroom | Engineering/SRE | [performance baseline](../performance-baseline.md), [capacity ADR](../architecture/adr-0012-bounded-transfer-capacity.md), [resilience evidence](../release-evidence/phase-5-resilience.md) | `PASSED` | Requalify before any limit/topology change | Measured capacity | Preserve 30 attempts/s and 1,800/minute tenant controls; repeat in the managed environment before partner traffic |
| G-020 | Phase 2, T094, TASK-013 | Real phone, tablet, laptop, and desktop journeys preserve the same financial meaning | Product UI/accessibility owner | [device matrix](../release-evidence/ui-device-matrix.md) and [executable runbook](../runbooks/physical-device-accessibility.md) | `BLOCKED_EXTERNAL` | Before partner traffic | Physical devices/reviewer | Commit-bound manifest/checklist tooling is ready; supply an authorized device farm or named reviewers, then execute, validate, and sign every row |
| G-030 | Phase 3, T095, TASK-014 | Finance-approved balance, aggregation, status, provenance, and UTC language | Finance + product | [financial semantics](../product/financial-ui-semantics.md), [decision register](../product/pilot-decision-register.md) | `BLOCKED_EXTERNAL` | Before shared overview/write use | Human decision | Review prepared definitions and record named approval, date, evidence, and expiry |
| G-031 | Phase 3, TASK-015 | Least-privilege roles, INR limits, negative-balance rule, destination policy, and pause authority are signed and enforced | Product + security/risk | [India launch profile](india-launch-profile.md), [decision register](../product/pilot-decision-register.md) | `BLOCKED_EXTERNAL` | Before production writes | Human decision | Approve or amend proposed values; Codex then binds the signed revision to policy tests |
| G-040 | Phase 4 | Named people detect, pause, communicate, recover, and reopen safely in seven incidents | Operations incident owner | [tabletop pack](../runbooks/operational-tabletop.md) | `BLOCKED_EXTERNAL` | Before partner traffic and every major response change | Participants/alert routes | Schedule named product, engineering, operations, security, support, and decision authorities |
| G-050 | Phase 5 | Technical target is AWS Mumbai/Cognito/RDS/ElastiCache with Hyderabad backup copy | Platform + product | [India launch profile](india-launch-profile.md), [threat model](../security/LedgerSync-threat-model.md) | `READY` | Architecture review before deployment | Internal preparation | Convert the selected technical baseline into reviewed IaC and cost/exit records |
| G-051 | Phase 5 | India jurisdiction, non-custodial wording, data classes, retention/deletion, and provider responsibility are legally authorized | Legal/compliance + product | [decision register](../product/pilot-decision-register.md) | `BLOCKED_EXTERNAL` | Before provider data or partner contract | Legal/corporate authority | Record counsel-approved boundary and retention decision reference |
| G-052 | Phase 5 | AWS account, region, budget, billing, DNS, and time-bounded automation access are authorized | User/platform owner | No credentials or authorization supplied | `BLOCKED_EXTERNAL` | Before resource creation | Provider access/budget | Provide approved account mechanism and budget ceiling without placing secrets in Git/chat/docs |
| G-060 | Phase 6, TASK-016 | Real Cognito login, tenant mapping, workload renewal/revocation, and secret rotation pass | Security + platform | [managed-environment gate](../release-evidence/phase-7-managed-environment.md) | `BLOCKED_EXTERNAL` | Before shared environment | IdP/secret access | Create and test real clients/tokens after G-031, G-051, and G-052 |
| G-070 | Phase 7, TASK-017 | Isolated private AWS environment, telemetry, alerts, and zero critical/high findings exist | Platform + security + operations | [managed-environment gate](../release-evidence/phase-7-managed-environment.md) | `BLOCKED_EXTERNAL` | Before shared traffic | Cloud access/owners | Apply reviewed IaC, deploy exact signed SHA, test reachability and alert delivery |
| G-080 | Phase 8, TASK-018 | Provider PITR meets signed RPO/RTO; clean Redis rebuild and reconciliation show zero unexplained mismatch | Operations + finance | [restore runbook](../runbooks/restore.md), provider evidence pending | `BLOCKED_EXTERNAL` | Before partner traffic; repeat at approved cadence | Managed backup history | Run isolated provider restore only after G-070 |
| G-090 | Phase 9, TASK-019 | Machine preflight accepts only current, owned, unexpired evidence for the deployed SHA | Product + engineering + finance + security + operations + legal | [preflight gate](../release-evidence/phase-7-managed-environment.md) | `BLOCKED_EXTERNAL` | Immediately before onboarding | Upstream approvals/evidence | Build final manifest and run `pilot-preflight --require-restore` after G-010–G-080 pass |
| G-100 | Phase 10, TASK-020 | One consenting design partner is provisioned under minimal signed limits with pause/offboarding proof | Partner owner + product | [controlled-pilot status](../release-evidence/phase-8-controlled-pilot.md) | `BLOCKED_EXTERNAL` | After G-090; before credentials | Partner contract/contacts | Name and authorize one partner, then dry-run deterministic provisioning and rollback |
| G-110 | Phase 11, TASK-021 | Approved live evidence window completes with zero duplicates, unexplained mismatches, or authorization breaches | Operations + partner owner | [controlled-pilot status](../release-evidence/phase-8-controlled-pilot.md) | `BLOCKED_EXTERNAL` | During the approved window | Live traffic/on-call | Start only after G-100 and preserve daily/weekly evidence |
| G-120 | Phase 12, TASK-022 | Named authorities select graduate, extend, remediate, or stop from actual evidence | Product + engineering + finance + security + operations + legal + partner | [graduation scorecard](graduation-scorecard.md) | `BLOCKED_EXTERNAL` | End of approved observation window | Upstream/live evidence | Populate and sign the scorecard; no blocked safety/legal row may graduate |

T121 is the Spec Kit umbrella for G-060 through G-120. It stays open until the actual managed topology, recovery, accountable approvals, and one partner release candidate exist.

## Task truth at Phase 0 exit

| Source | Complete | Open | Open identifiers |
|---|---:|---:|---|
| Spec Kit `tasks.md` | 118 / 121 | 3 | T094, T095, T121 |
| Master implementation plan | 12 / 22 | 10 | TASK-013 through TASK-022 |

The two lists overlap; their counts must not be added together. TASK-013 maps to T094, TASK-014 maps to T095, and T121 summarizes multiple managed/human/partner tasks.

## Maintenance evidence

GitHub’s earlier official actions used deprecated Node runtimes. On 2026-08-24 the repository verified the publishers’ current releases and pinned immutable commit SHAs for:

- [checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1);
- [setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0);
- [setup-node v7.0.0](https://github.com/actions/setup-node/releases/tag/v7.0.0);
- [upload-artifact v7.0.1](https://github.com/actions/upload-artifact/releases/tag/v7.0.1);
- [setup-buildx v4.3.0](https://github.com/docker/setup-buildx-action/releases/tag/v4.3.0);
- [attest-build-provenance v4.2.2](https://github.com/actions/attest-build-provenance/releases/tag/v4.2.2).

Those revisions use Node 24 or a pinned composite action. The Phase 0 workflow run is the compatibility proof. Separately, `npm ci` reports that resolved ESLint 9.39.5 is outside upstream support; upgrading the lint stack is non-blocking maintenance and must be tested with Next.js before changing the lockfile.

## Update protocol

1. Change a status only with a linked result, named owner, and UTC date.
2. A failed gate moves to `FAILED_REMEDIATE`; do not soften it to partial/conditional pass.
3. Missing or expired external evidence is `BLOCKED_EXTERNAL`.
4. Keep secrets, raw tokens, customer data, and unredacted logs outside Git.
5. After every phase, update the evidence, task checkbox, this row, commit with bullet-body context, push, and require green workflows.
