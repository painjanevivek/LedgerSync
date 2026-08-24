# LedgerSync local-only MVP gate register

**Status:** authoritative for the ready-to-use product on one workstation

**Boundary:** one Windows workstation, Docker Desktop, loopback-only web access, INR demo data, and internal same-currency ledger transfers

**Not a claim of:** production deployment, custody, bank connectivity, managed identity, provider backup, legal approval, or design-partner readiness

This register separates the product that can be completed and used locally from the controlled production-pilot program. A local pass proves the repository and local runtime behave correctly inside the boundary above. It does not overwrite or weaken historical pilot evidence in [the production-pilot register](completion-gates.md).

## Status rules

| Status | Meaning |
|---|---|
| `READY` | Inputs exist and the gate can be executed locally. |
| `IN_PROGRESS` | Work or verification is currently underway. |
| `FAILED_REMEDIATE` | Evidence failed; the local MVP is not ready until repaired and retested. |
| `PASSED` | Every local exit criterion has reproducible evidence. |
| `OUT_OF_SCOPE_LOCAL_MVP` | The requirement belongs to a shared or production environment and cannot block this loopback-only release. It remains tracked in the production-pilot register. |

No partially passing state is allowed. A failed financial invariant blocks the local release.

## Local gates

| ID | Outcome in plain language | Required evidence | Status |
|---|---|---|---|
| L-010 | One safe command recovers the complete stack and preserves existing ledger data | [Phase 1 runtime evidence](../release-evidence/local-mvp-phase-1-runtime.md) | `PASSED` |
| L-020 | Every visible overview, account, transfer, and reconciliation control uses the real local API | [Phase 2 operator-workspace evidence](../release-evidence/local-mvp-phase-2-operator-workspace.md) | `PASSED` |
| L-030 | Exact money, idempotency, immutable double entry, authorization, read-your-writes, and reconciliation remain correct | [Phase 3 transfer-safety evidence](../release-evidence/local-mvp-phase-3-transfer-safety.md) | `PASSED` |
| L-040 | The local operator can back up, restore, rebuild disposable cache state, and explain recovery | [Phase 4 recovery evidence](../release-evidence/local-mvp-phase-4-recovery.md) | `PASSED` |
| L-050 | The workspace is usable and understandable on desktop, tablet, and mobile viewports | [Phase 5 responsive and accessible web evidence](../release-evidence/local-mvp-phase-5-web-quality.md) | `PASSED` |
| L-060 | Loopback exposure, demo identity, secret handling, and browser/API boundaries are truthful and fail safely | Boundary inspection, dependency/security checks, negative-path tests | `READY` |
| L-070 | A clean-machine-style startup and full acceptance run produce one reviewable local release result | Consolidated acceptance report tied to a Git commit | `READY` |

## Explicitly outside this local release

The following retain their existing `BLOCKED_EXTERNAL` or other historical status in the production-pilot register and are `OUT_OF_SCOPE_LOCAL_MVP` here:

- physical-device or external device-farm sign-off;
- cloud infrastructure, public DNS, and internet deployment;
- managed OIDC/SSO and real organization lifecycle;
- provider-backed PITR and multi-region recovery;
- legal, compliance, custody, and contract decisions;
- named on-call responders and external alert destinations;
- partner credentials, live customer data, and production traffic.

## Update protocol

1. Change a local gate to `PASSED` only after its commands and human-readable outcome are recorded.
2. Never use local demo evidence to change a production-pilot gate.
3. Never commit secrets, session cookies, raw balances, database dumps, or unbounded logs.
4. Stop immediately on reconciliation mismatch, duplicate movement, authorization leak, or unexplained ledger/projection difference.
