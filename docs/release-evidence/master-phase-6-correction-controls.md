# Master Phase 6 — correction and approval controls

**Qualified candidate:** `64d3fe8c35f26ca915d8ec4011f2039ac372d123`

**Captured:** 2026-08-28 (Asia/Calcutta)

**Decision:** Phase 6 repository exit gate passed; Phase 7 may begin

## Delivered financial control

- Added immutable, additive transfer compensation commands that preserve the original transfer, original journal, approval record, policy evidence, and audit history.
- Added a fixed correction reason taxonomy, bounded operator notes, approval expiry, requester cancellation, approver rejection, and terminal-state conflict protection.
- Enforced requester, approver, and auditor role separation with object authorization, explicit correction scopes, and production step-up authentication for approve, reject, and post commands.
- Snapshotted the active correction policy version and approval window so every decision remains explainable under the rules that governed it.
- Posts one exact reverse transfer and its balanced journal through the existing serializable transfer path; partial, repeated, and cross-tenant compensation remain impossible.
- Links original and compensating transfers bidirectionally in detail, history, explainability, audit, and schema-versioned CSV export without rewriting posted evidence.
- Blocks freeze, reactivate, and close transitions when unresolved corrections or financial obligations exist, while preserving authorized history after terminal closure.

## Operator and API product

- Published scoped private routes for correction request, list, detail, approve, reject, cancel, and post operations.
- Advanced the reviewed OpenAPI contract and safe developer metadata with fixed routes, scopes, bounded errors, retry outcomes, policy snapshots, and exact identifiers.
- Added a same-origin BFF boundary with strict Host/origin/CSRF checks, bounded JSON allowlists, signed actor assertions, explicit recent-reauthentication evidence, timeouts, and sanitized unknown-outcome responses.
- Added a progressively disclosed correction workspace with a review queue, immutable evidence detail, policy and approval timeline, guarded sensitive commands, and no direct ledger mutation surface.
- Keeps compensated and closed lifecycle states inspectable, exportable, and linked to the same authorized financial evidence.

## Commit chain

| Commit | Purpose |
|---|---|
| `b8f86fe` | Define versioned correction policy and decision evidence. |
| `443f97b` | Advance fresh-schema diagnostics for the new controls. |
| `0b288f6` | Post exact additive transfer compensations. |
| `4956e7c` | Expose step-up protected correction commands. |
| `f642e94` | Preserve compensated lifecycle and export evidence. |
| `7403b81` | Add the correction review workspace and browser flows. |
| `71363e7` | Qualify correction controls and promote reviewed Linux baselines. |
| `c191d4e` | Correct PostgreSQL approval persistence and test setup. |
| `cbfbf72` | Align lifecycle fixtures with financial ownership policy. |
| `8e61321` | Preserve same-currency lifecycle evidence. |
| `64d3fe8` | Authorize the owned round-trip closure history fixture. |

## Verification

| Gate | Result |
|---|---|
| Go application/platform/unit/contract | Passed locally with `go test ./... -count=1` and `go vet ./...`, including correction domain, policy, repository, handler, lifecycle, export, and OpenAPI drift paths. |
| Live PostgreSQL integration | Passed locally and in exact-candidate CI: request/replay, role separation, expiry, cancel/reject, recent step-up, exact reverse posting, balanced paired journals, repeated/partial prevention, authorization, policy snapshots, preserved history, and lifecycle safeguards. |
| Web security/unit | Passed 94/94 with strict BFF request and response boundaries. |
| Correction browser journeys | Passed 2/2 for progressively disclosed review and guarded sensitive operations. |
| Accessibility and responsive checks | Passed locally 19/19 with 44 CSS-pixel controls, compact reflow, keyboard operation, and no serious or critical axe findings. |
| Visual review | Passed the exact Linux browser matrix after inspecting and approving 13 intentional baseline changes limited to the Corrections navigation destination; the approval is recorded in the visual baseline ledger. |
| Production-path CI | Passed exact-candidate run `33173619668`. |
| Quality and isolated real stack | Passed exact-candidate run `33173619571`: formatting, vet, static analysis, race, fuzz, coverage, web lint/test/build/budgets, browser journeys, reviewed visuals, disposable PostgreSQL/Redis integration, retry after restart, seed idempotency, PostgreSQL restart and reconciliation, digest-bound backup and isolated restore, dependency-fault recovery, hardened containers, and commit-bound release evidence. |
| Supply chain and security | Passed exact-candidate run `33173619613`: secret history, dependency audit, IaC scan, immutable container scans, SBOMs, and provenance attestations. |

## Manual activation boundary

The repository gate is complete, but production authority is not self-activated. A named finance and security authority must approve role assignments, correction reason policy, approval duration, step-up window, and operator separation during tenant provisioning. Legal and custody-language confirmation remains a Phase 14 gate.

## Exit-gate decision

- Every supported correction is additive, exact, balanced, independently authorized in production, policy-versioned, and explainable from immutable evidence.
- Partial or repeated compensation cannot produce a second financial movement.
- Account lifecycle commands cannot bypass unresolved correction or financial obligations, and authorized history remains available after closure.
- No supported production correction workflow requires direct SQL, balance editing, journal rewriting, or ledger mutation.
