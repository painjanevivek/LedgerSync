# Master Phase 4 — guided operator journey foundation

**Result:** `FOUNDATION PASSED / PHASE ACTIVE`

**Candidate:** `8800eafa783f6f5d2ded105b03a498c1dcab75c2`; the implementation, reviewed visual baselines, migration, staged-role compatibility, and tests are pushed to `origin/main`.

**Completion boundary:** this evidence does not close Phase 4. The full first-run loop depends on the approved funding-journal capability in Phase 5. LedgerSync truthfully renders that step as unavailable and never treats a transfer, seed balance, or database edit as funding.

## Implemented product outcome

- Replaced the seven-row guide with the ordered twelve-step journey from health through verified backup.
- Added one context-aware recommended action while preserving the current workspace and existing return context.
- Separated durable stored evidence, evidence ready for inspection, explicit operator confirmation, missing evidence, and unavailable capability.
- Added PostgreSQL-owned tenant-and-subject preferences for dismissal and allowlisted manual confirmations.
- Added optimistic versioning, conflict refresh, bounded strict request parsing, `local:write`, operator-role enforcement, CSRF protection at the BFF, and fixed private routing.
- Added dismiss, reopen, restart-manual-progress, safe-stop, compact layout, forced-colors borders, and plain-language definitions.
- Kept funding unavailable pending Phase 5 rather than fabricating a successful end-to-end loop.

## Verification performed on the candidate

| Gate | Result |
|---|---|
| Go command, application, platform, unit, and contract regression | Passed: `go test ./cmd/... ./internal/... ./tests/unit/... ./tests/contract/...` |
| Focused guidance/API contracts | Passed, including twelve-step ordering, preference prerequisites, malformed input, scope denial, and version conflict |
| Web unit/security | Passed: 85/85 |
| Type/lint/production build | Passed |
| Browser functionality, accessibility, responsive, and visual suite | Passed 124/124 after reviewed baseline promotion; includes unknown-response refresh without optimistic completion |
| Reviewed cross-platform visual changes | Populated and mixed-currency Overview images inspected on Windows and from exact-commit pinned-Linux CI; hierarchy, stop-ship semantics, exact evidence, navigation, and error separation approved |
| Web performance | Passed 2/2; compact Overview LCP 1868 ms, CLS 0, observed INP 72 ms |

## Exact-commit hosted evidence

| Gate | Run | Result |
|---|---|---|
| Quality | `33156046449` | Passed for `8800eaf`: Go, web, 124 browser checks, reviewed Linux visuals, disposable PostgreSQL/Redis integration, migration upgrade compatibility, role privileges, real-stack recovery, and release evidence |
| Production path | `33156046450` | Passed for `8800eaf` |
| Supply chain and security | `33156046469` | Passed for `8800eaf` |
| Contract validation | `33155048723` | Passed for `3370d5c`; the later commits changed only database role provisioning and stale integration evidence, outside the contract workflow path filter |

The corrective trail is preserved rather than hidden: `d4fc49d` introduced the foundation, `3370d5c` canonicalized empty progress and promoted reviewed Linux images, `acb0847` added least-privilege preference grants and the migration-17 diagnostics expectation, and `8800eaf` made those grants valid in both supported provisioning orders. The final hosted upgrade simulation proves that `roles.sql` remains safe on pre-migration-17 schemas.

## Remaining Phase 4 exit work

- Complete and approve Phase 5 controlled funding journals.
- Replace the funding capability stop with authorized durable funding evidence and its safe next action.
- Execute the complete first-time health-to-backup loop after funding exists.
- Re-run exact-commit local, Linux visual, integration, security, production-build, and real-stack gates before changing M04 to `COMPLETE`.
