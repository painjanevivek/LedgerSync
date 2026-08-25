# LedgerSync Phase 11 cleanup audit

**Audit mode:** Evidence only; no deletion, extraction, split, dependency edit, route edit, or runtime cleanup was performed  
**Repository:** `D:\Work\Project\Dev\LedgerSync`  
**Audited source:** `main` at `69f89319f1cb4624d97ee23160a25a60a2ce686f`  
**Decision rule:** Anything below 90% confidence is retained  
**Next gate:** Phase 12 may start only after the user approves named rows from this document

## 1. Executive decision

The supported LedgerSync product is not filled with disposable application code. Its active Go module, Next.js application, API routes, environment contract, Compose runtime, and production dependencies are all exercised by builds, tests, configuration, or external protocol entry points.

The strongest cleanup opportunity is an archived predecessor implementation: `backend/`, `dashboard/`, `simulation/`, `setup/`, `docker-compose.legacy-demo.yml`, and `tests/consistency_test.go`. It is excluded from the supported runtime, incomplete, and non-runnable. It must be removed as one coordinated slice, not file by file, because current documentation and a production-boundary contract intentionally refer to it as excluded history.

A small set of supported-code symbols and two web utilities are also proven unused. The remaining recommendations are responsibility-preserving extractions and splits. High-risk financial modules remain retained where confidence is below 90%.

## 2. How evidence was gathered

- Enumerated all 697 tracked paths with `git ls-files`, then used `rg --files` and exact symbol searches to trace imports, call sites, route registrations, manifests, Compose profiles, scripts, workflows, tests, and documentation references.
- Ran the supported root package inventory, `go test ./...`, `go vet`, static analysis, and test-aware dead-code analysis. The supported root tests and vet passed.
- Ran TypeScript resolution, ESLint, 78 web unit/security tests, and the Next.js production build. All passed; the compiled build contained all 23 API routes and all 15 authored page routes.
- Compared `.env.example` variables with code, Compose, script, workflow, and test consumers. Every declared variable has a real consumer.
- Compared root Go dependencies with imports and `go mod why`; checked web dependencies against source, scripts, CI, test tooling, and the lockfile.
- Searched active source for commented-out implementation statements, `TODO`, `FIXME`, placeholder controls, fake runtime behavior, and unreachable route families.
- Inspected ignored and untracked directories by path, ignore rule, producer, size, and file metadata. No local evidence directory was deleted or modified.
- Counted production-file size and inspected high-complexity responsibilities before recommending a split. File length alone was not treated as proof.

## 3. Approved-deletion candidates

No row in this table has been executed.

| ID | Candidate | Evidence of genuine non-use | Confidence | Behavior risk | Recommendation | Required verification after approval |
|---|---|---|---:|---|---|---|
| D-01 | Atomic archived demo slice: `backend/`, `dashboard/`, `simulation/`, `setup/`, `docker-compose.legacy-demo.yml`, `tests/consistency_test.go` | The 30 tracked files are absent from `go.work`, supported Compose, images, workflows, and normal tests. The legacy Compose file is their only runtime relationship. It references missing `setup/redis-config.conf`; generated proto Go files and a dashboard lockfile are absent. The backend modules have missing sums, invalid syntax/imports, and cannot generate or compile the referenced proto. The tagged test is explicitly a placeholder and cannot compile because its proto package does not exist. README and the threat model describe this slice as excluded history. | 98% | Low supported-product risk; medium historical-document risk | Delete only as one coordinated slice. Update current README/threat-model wording and invert the production-boundary contract to assert absence. Preserve historical review documents as records. | `go mod tidy`; root tests/vet/static analysis; contract tests; canonical and restore Compose validation; web install/lint/unit/build; local acceptance smoke. |
| D-02 | `deploy/compose/docker-compose.yml:215-226` — `legacy-simulation` diagnostic profile | It is a sleep-only container with no application mount, endpoint, or action. Repository search found no supported `--profile diagnostic` invocation. Authenticated `/api/local/diagnostics` is the real diagnostic product. | 99% | Low | Delete the service/profile. Do not remove the real diagnostic API, BFF, tests, or UI. | Compose configuration and security contracts; local start/status; diagnostics browser and Redis-degradation tests. |
| D-03 | `internal/application/transfers/authorize.go` — `AuthorizeDebit`, `AuthorizeCredit`, then the empty file | Test-aware dead-code analysis reports both unreachable. Exact searches find only definitions/comments. The comment claiming the PostgreSQL adapter calls the debit helper is false; the atomic repository applies its own locked-account policy. | 99% | Medium because the names imply financial policy | Delete both functions/file without altering repository validation. | Domain account tests; transfer authorization, atomicity, idempotency, concurrency, fault, pilot-control, and real-stack suites. |
| D-04 | `internal/domain/account/account.go:99` — `Account.CanRead` | Test-aware dead-code analysis and exact repository search find only the declaration. Production and tests use the active authorization paths instead. | 99% | Low | Delete the method. | Domain account and unit authorization tests; full Go test. |
| D-05 | `internal/application/operations/diagnostics.go:42` — `DependencyStatus` | Exact whole-repository search finds only its declaration. `DiagnosticSnapshot` uses the narrower PostgreSQL, reconciliation, outbox, and Redis status types. | 99% | Low | Delete the type. | Operations application and HTTP-handler tests; full Go test/static analysis. |
| D-06 | `internal/platform/db/postgres.go:51` — exported `WithSerializableRetry` wrapper | Test-aware dead-code analysis and exact search find no call. The private `withSerializableRetry` remains used by `WithSerializableSequence`; only the unused wrapper is removed. | 99% | Low | Delete the exported wrapper and its stale public comment; retain the private implementation. | Database unit/integration conflict tests and full transfer concurrency suite. |
| D-07 | Five legacy `WithBFFAssertionSecret` handler setters in accounts, balances, transfers, transactions, and investigation handlers | Exact repository search finds definitions only. Canonical API composition uses `WithRequestAuthenticator` for all five handlers. | 98% | Low | Delete the five unused fluent setters atomically. | All handler tests, API composition, actor-assertion, replay, scope, and fail-closed tests. |
| D-08 | `web/src/features/console/format.ts:14` — `shortIdentifier` | Exact symbol search across `web/src` and `web/tests` finds only the exported definition. ESLint cannot flag it because it is exported. | 99% | Low | Delete the function; retain the file's used account and UTC formatters. | Web lint, unit tests, TypeScript build, visual tests. |
| D-09 | `web/src/lib/api/recovery.ts:66` — `sanitizeRecoveryBody` | Exact symbol search finds only its exported definition. The live recovery route imports `sanitizeRecoveryIndex` and the shared bounded proxy performs parsing. | 99% | Low | Delete only `sanitizeRecoveryBody`. | Recovery/export security tests, recovery E2E, TypeScript build. |
| D-10 | Root direct dependencies `github.com/go-redis/redis/v8`, `google.golang.org/grpc`, and `github.com/stretchr/testify` after D-01 | Redis v8, gRPC, and testify are imported by the broken legacy-tagged `tests/consistency_test.go`; canonical code uses Redis v9 and does not use gRPC/testify. This row depends on D-01 and must not be executed independently. | 98% after D-01 | Low | Run `go mod tidy` after D-01 and review the exact manifest/lock diff. | Clean module download; root test/vet/static analysis; SBOM and vulnerability qualification. |

### D-01 exact tracked scope

The atomic legacy slice contains:

- 13 files under `backend/`, including four independent modules and `backend/proto/balance.proto`;
- 10 files under `dashboard/`;
- 4 files under `simulation/`;
- `setup/postgres-setup.sql`;
- `docker-compose.legacy-demo.yml`;
- `tests/consistency_test.go`.

No individual item inside this group should be approved without the complete D-01 documentation and contract update.

## 4. Unused imports, variables, functions, and exports

| Area | Evidence | Decision |
|---|---|---|
| Active Go imports and variables | The supported packages compile, vet, and pass static analysis. No unused import or local-variable finding exists. | No deletion candidate. |
| Active TypeScript imports and variables | TypeScript production build and ESLint pass across source/tests. | No deletion candidate. |
| Go exported symbols | Test-aware dead-code analysis produced D-03 through D-07 plus the retained sub-90% domain chain below. | Delete only approved named rows. |
| TypeScript exported symbols | Exact occurrence analysis found D-08 and D-09. `web/src/instrumentation.ts#register` also has no literal importer, but it is a Next.js convention entry point and is retained. | D-08/D-09 are candidates; convention roots are retained. |

## 5. Dependency audit

| Manifest | Evidence | Decision |
|---|---|---|
| Root `go.mod` | Canonical direct dependencies are imported by production or supported tests. The three legacy-only dependencies are isolated as dependent row D-10. | No independent dependency deletion. |
| `web/package.json` | Every runtime dependency is imported. Playwright, Axe, ESLint, TypeScript, `tsx`, and type packages are used by build/test scripts. `@redocly/cli` is executed by contract CI and pinned by a supply-chain contract. | Retain all. |
| Archived `dashboard/package.json` and four backend modules | They belong to D-01 and are not independently supported dependency surfaces. | Remove only with D-01. |

## 6. Environment, routes, and endpoint audit

| Category | Evidence | Confidence | Risk | Decision |
|---|---|---:|---|---|
| `.env.example` | Every declared variable has at least one code, Compose, workflow, script, test, or contract consumer. Single-consumer HTTP limits, rotation keys, OIDC settings, and resource audience are real configuration reads. | 96% retain | High configuration compatibility | No unused environment variable. |
| 23 Next.js API routes | All are present in the compiled app-path manifest. Business route families have UI/test consumers; authentication callback routes are external OIDC entry points and cannot be proven unreachable by literal-link search. | >90% retain | High/critical | Retain all. |
| 15 authored pages | All are compiled. `/admin` is intentionally unlinked and returns 404 as a deny-by-default security boundary. `/sign-in` and callback routes remain external-protocol surfaces. | >90% retain | High/critical | Retain all. Add an explicit `/admin` 404 browser assertion if routing is changed. |
| Private Go routes | Registered routes match OpenAPI/BFF families; health/readiness and private OpenAPI assets have runtime, test, or operational consumers. | 96% retain | High | No unused endpoint. |

## 7. Commented-out code and placeholders

| Finding | Evidence | Confidence | Risk | Decision |
|---|---|---:|---|---|
| Active supported source | Searches found explanatory comments, intentional empty catch comments, test fakes, and input placeholder attributes, but no commented-out implementation block or non-functional product control. | 96% | Low | No action. |
| `tests/consistency_test.go` | The legacy-tagged file literally describes itself as a placeholder and lists what a real test would do. | 99% | Low supported-product risk | Included in D-01. |
| `legacy-simulation` | The container only prints an exclusion message and sleeps forever. | 99% | Low | Included in D-02. |

## 8. Duplicated active logic

| ID | Candidate | Duplication evidence | Confidence | Risk | Recommendation | Required verification |
|---|---|---|---:|---|---|---|
| X-01 | Eleven handler `authenticate` implementations | Account commands, accounts, balances, developer contracts, guidance, investigation, operations, reconciliation commands, recovery exports, transactions, and transfers repeat the same provider/authenticator selection body. | 98% | Medium authentication behavior | Extract one package-private `authenticateRequest(provider, authenticator, request)` helper. Preserve receiver wrappers only where they aid readability. | Every handler suite plus actor assertion, workload identity, scope, operator-role, and fail-closed tests. |
| X-02 | Console session, online-state, and sign-out effects | `OperatorConsole`, `OperationsConsole`, `DeveloperConsole`, and `RecoveryConsole` independently fetch `/api/session`, subscribe to browser online/offline events, post `/api/auth/sign-out`, and refresh routing. | 96% | Medium auth/offline lifecycle | Extract narrowly scoped `useConsoleSession` and `useOnlineStatus` hooks. Keep domain loading, denial, and screen copy local. | Session security tests and all account, transfer, reconciliation, operations, developer, recovery, responsive, and accessibility journeys. |
| X-03 | Account/reconciliation mutation boundary header helpers | The two modules repeat private request headers, public response-header allowlists, and dispatch-error mapping while their scopes, idempotency validation, and unknown-outcome codes differ. | 94% | High mutation security | Extract only byte-for-byte equivalent header/filter helpers; retain domain-specific policy in each boundary. | Account and reconciliation mutation security plus request-boundary tests and real browser mutation journeys. |
| X-04 | Replay CLI setup in `cmd/replay-outbox` and `cmd/replay-delivery` | The commands are structurally similar, but domain actions and result semantics differ; only two consumers exist. | 84% — retain | Medium recovery operations | Do not extract yet. Reassess if a third replay command appears. | Not applicable until a later design decision. |
| X-05 | Small `isRecord`, UUID, bounded-string, idempotency, and sanitizer validators | Names and shapes recur, but allowlists, size limits, status semantics, and security responses differ by domain. Equivalence is not proven. | 70% — retain | High validation/security coupling | Keep local. Consolidate only after contract tests prove exact semantic identity. | Not approved for Phase 12. |

## 9. Oversized files and responsibility splits

| ID | File | Evidence | Confidence | Risk | Recommendation | Required verification |
|---|---|---|---:|---|---|---|
| S-01 | `internal/application/exports/service.go` — 330 lines | Mixes three pagination workflows with CSV quoting, formula neutralization, text safety, and filter fingerprints. Reconciliation streaming has high branch complexity. | 95% | Medium evidence correctness | Extract only CSV formatting/fingerprinting into package-local `csv.go`; keep orchestration and public methods unchanged. | Export unit/integration and recovery-export handler suites. |
| S-02 | `internal/transport/http/handlers/recovery_exports.go` — 397 lines | Combines four endpoints, authentication, authorization, audit, query parsing, response headers, streaming state, and a cohesive delayed writer. | 94% | Medium header/audit failure semantics | Split query helpers and delayed writer into package-local files; preserve handler public API. | Complete recovery/export handler and integration suites. |
| S-03 | `web/src/features/accounts/OperatorConsole.tsx` — 212 physical lines with dense orchestration | One composition root owns account, transfer, reconciliation, explainability, orientation, session, online, routing, and initial-load state for nine page shapes. | 94% | Medium/high shared workspace state | Extract domain hooks; retain `OperatorConsole` as the public composition boundary. | Full web unit, account/transfer/reconciliation/orientation E2E, responsive, accessibility, visual, and performance suites. |
| S-04 | `web/src/lib/api/operations.ts` — 288 lines | One module owns diagnostics and event DTOs plus page/detail/body sanitizers. Live routes already import distinct public functions. | 93% | High allowlisting/redaction | Split diagnostics and event sanitizers behind unchanged public exports or an atomic import update. | Operations-read security, operations/recovery E2E, hostile redaction, real-stack evidence. |
| S-05 | `web/src/app/globals.css` — 582 lines | Global shell, financial workspace, transfer, reconciliation, operational, developer, recovery, and Phase 9 typography rules share one cascade. Tokens and responsive rules are already separate. | 92% | Medium/high cascade and visual drift | Split by stable responsibility while preserving exact import order, selector specificity, and tokens. | Linux/Windows visual baselines, responsive matrix, Axe, keyboard, forced colors, reduced motion, and performance. |
| S-06 | `web/src/features/reconciliation/ReconciliationCommand.tsx` — 257 lines | Component mixes durable intent, polling, unknown-outcome handling, review state, and evidence rendering. | 91% | High idempotency/polling state | Extract only the polling/submission state machine into a hook. | Reconciliation intent unit tests, complete command E2E, accessibility. |
| S-07 | `scripts/local-backup-common.ps1` — 898 lines | Combines safe container byte streaming, root/child containment, manifest validation, retention, recovery index generation, and filesystem safety. These are distinct cohesive responsibilities with strong script tests. | 93% | High backup custody | Split into dot-sourced private modules while keeping public script function names unchanged. | Recovery-evidence script tests, live protected backup, isolated restore, acceptance recovery, corruption/path-containment guards. |
| S-08 | `internal/platform/db/transfer_repository.go` — 591 lines | Mixes locked-account validation, idempotency, financial transaction orchestration, posting persistence, audit, outbox, and result construction. Its core path is high complexity but strongly tested. | 88% — retain | Critical financial atomicity | Do not split in Phase 12. Design a transaction-boundary-preserving mechanical split first. | Not approved under the 90% rule. |
| S-09 | `cmd/api/main.go` and `internal/platform/config/config.go` | Startup composition and configuration loading are large/high-complexity, but fail-closed ordering and readiness semantics make a mechanical boundary uncertain. | 88% — retain | High startup/security defaults | Retain until explicit composition and cross-field validation seams are designed. | Not approved under the 90% rule. |

`contracts/openapi.yaml` is 967 lines but intentionally remains one authoritative machine-readable contract. Its length is not a split defect.

## 10. Generated, temporary, and user-owned local state

No row in this section was touched.

| Path | Evidence | Delete confidence | Risk | Decision |
|---|---|---:|---|---|
| `tmp/` | Exact root directory is empty, untracked, unignored, and has no tracked producer. Tracked tools use `.tmp/`, OS temp, or container `/tmp`. | 99% technically temporary | Negligible | Retain untouched because it has no Git impact and the user previously directed that identified temporary files not be touched. |
| `.tmp/` | Ignored by `.gitignore:37`; current contents are generated qualification artifacts including capacity results, SBOMs, scan output, image archives, coverage, Playwright traces, and evidence JSON. Some may be the only local copy. | 84% | Medium/high evidence and confidentiality loss | Retain under the 90% rule. Never bulk-delete. Define targeted retention/archive policy first. |
| `data/` | Ignored by `.gitignore:33`; contains generated credentials, local database/runtime state, protected backups, restore evidence, and acceptance state. | 15% | Critical secret, restore, and local-data loss | Retain. Only exact lifecycle scripts may operate on exact targets. |
| `web/.next`, `web/node_modules` | Ignored and reproducible from source/lockfile. | 99% | Low, except offline rebuild time | Not a repository change. Retain unless the user separately asks for disk cleanup. |
| Playwright reports/results | Ignored and recent; may preserve failure or visual evidence. | 88% | Medium evidence loss | Retain under the 90% rule. |
| `.cache/`, `.tools/` | Ignored local toolchain/cache state; reproducible online, but offline rebuild cost is uncertain. | 85% | Medium workflow disruption | Retain under the 90% rule. |
| `output/pdf/LedgerSync-Product-and-Engineering-Report.pdf` | Tracked deliverable, not ignored runtime residue. | 0% | High document loss | Retain. |

## 11. Documentation relationships that constrain deletion

- README currently says the legacy Compose file is intentionally retained for history.
- `docs/security/LedgerSync-threat-model.md` names `backend/`, `dashboard/`, `simulation/`, and legacy Compose as excluded from the production boundary.
- The production-boundary contract asserts the legacy slice cannot leak into production artifacts.
- Release evidence records `/admin` as a deliberate deny-by-default route and `/sign-in` as a local-demo redirect; absence of navigation links is not evidence of unused routing.
- Responsive baselines and contact sheets are direct visual-regression/review evidence, not arbitrary generated images.

D-01 therefore requires an atomic documentation and contract correction. Historical dated reviews should remain immutable even if they mention removed paths.

## 12. Non-cleanup correctness observations

These are not deletion approvals and should be handled as separately reviewed corrections.

| ID | Observation | Evidence | Recommendation |
|---|---|---|---|
| C-01 | Footer cache wording is stale | `ConsoleFooter` says “Cached reads are version-checked,” while customer-visible balance reads now always come from PostgreSQL and Redis is only warmed as disposable state. | Change the sentence to state that PostgreSQL supplies customer-visible balances and Redis is disposable. Re-run convergence, accessibility, visual, and unit tests. |
| C-02 | Broad Go package discovery enters ignored frontend dependencies | With `web/node_modules` present, `go list ./...` discovers `web/node_modules/flatted/golang/pkg/flatted`. | Use explicit supported Go package patterns in developer/quality commands, or create a deliberate module boundary; do not let ignored JavaScript dependencies change Go qualification scope. |

## 13. Approval packages and stop gate

For a small, low-risk first cleanup, approve:

- **Package A — proven dead supported symbols:** D-03, D-04, D-05, D-06, D-07, D-08, D-09.

For retired runtime scaffolding, approve separately:

- **Package B — no-op diagnostic profile:** D-02.
- **Package C — complete archived predecessor removal:** D-01 and dependent D-10.

For behavior-preserving maintainability work, approve rows individually:

- **Package D — duplicate extraction:** X-01, X-02, and/or X-03.
- **Package E — responsibility splits:** S-01 through S-07, individually named.

Rows X-04, X-05, S-08, S-09, and every sub-90% temporary/generated item are explicitly retained and cannot be inferred from a blanket approval.

**STOP:** Phase 12 has not started. Reply with exact row IDs or package letters to authorize the next change set. Corrections C-01 and C-02 require separate explicit approval because they are not cleanup deletions.
