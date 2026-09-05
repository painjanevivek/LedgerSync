# Live website smoke test — 5 September 2026

## Verdict

The redesigned application now starts, authenticates, and serves the inspected workflows on a real local stack. Four defects were reproduced and fixed. This is **not a declaration that every redesign or production acceptance gate is complete**.

Changes are in the `codex/003-simple-first-hardening` working tree at `.codex-tmp/main-integration`. They have not been committed, pushed, or deployed. Existing uncommitted work was preserved.

## Test boundary

- Inspected the existing application at `http://127.0.0.1:3000` first. It is an older build: `/tasks` returned 404 and the guide overflowed at 390px. It was not the redesigned source under review.
- Built a separate production-mode Next.js image and real Go API from the implementation working tree. The isolated Docker Compose project is `ledgersync-smoke-20260905`, exposed at `http://127.0.0.1:3300`, with its own PostgreSQL 16 database, Redis, network, and worker.
- Adjusted only the isolated stack's health probe to send its configured public host (`127.0.0.1:3300`) to the internal listener. The default port-3000 host correctly received 421 from host validation. No application host-validation bypass was added.
- Browser smoke interactions used real API responses, not Playwright route fixtures. Existing local credentials were reused in process memory without printing their values or editing secret files. No cloud resources were created.
- The smoke database started empty. Created one zero-balance account through the UI (`Smoke test operating account`). No money was added or transferred, no financial evidence was invented, and the normal ledger database was not used for financial mutations.
- The separately run regression suite uses controlled fixtures. Its pass count must not be represented as real-backend coverage.

## Reproduced defects and fixes

| Finding | Impact | Fix and verification |
|---|---|---|
| Clean Linux `npm ci` failed on missing `@emnapi/core` / `@emnapi/runtime` lock entries | Container could not build despite the existing Windows install working | Regenerated package-lock metadata with a clean Linux npm install; clean Docker install and Next.js production build passed |
| Migration 000037 and session consistency updates called nonexistent `jsonb_object_length` | PostgreSQL migration failed; API could not start | Use `jsonb_array_length(jsonb_path_query_array(value, '$.*'))` in both SQL locations; fresh migration succeeded; actual PostgreSQL repository regression passed |
| Optimized local build issued a `__Host-` session cookie without `Secure` | Browser discarded the cookie; sign-in loop | Centralized prefix and transport policy using the explicit deployment environment; real sign-in/sign-out/re-login passed, with production security-policy unit coverage |
| Tasks said “You’re all caught up” before data loaded and after failed/offline reads | False financial reassurance | Require verified, online, complete reads for all-clear; expose pagination incompleteness; real offline reproduction now shows “Tasks need to be checked”; added loading/error/pagination regressions |

Cookie inspection returned metadata only: a 43-character opaque value, HttpOnly, SameSite=Lax, Path=/, and the unprefixed local cookie name. Local HTTP uses the existing explicit development-only insecure-cookie setting. Production prefixes remain Secure. Sign-out removed the cookie and `/api/session` returned 401; re-login returned the authenticated Home page.

The PostgreSQL test uses the actual `ledgersync_api` role. It covers create/resolve, ten consistency requirements, rejection of an eleventh without losing the session, rotation, revocation, default Simple preferences, optimistic version conflicts, and operator preference isolation. It passed under Linux `-race`; this does not establish a full repository race pass.

## Browser coverage

- Loaded Home, Accounts, Add money, Transfers, Tasks, Approvals, Corrections, Reconciliation, Events, Webhooks, Developer, Recovery, System status, and Help on the real stack. The inspected desktop route sweep returned 200 without unexpected page errors or failed API reads.
- Switched Simple/Expert mode, observed successful preference writes, and reloaded to verify persistence.
- Exercised account creation through review and a successful API response; verified the resulting zero balance. Funding and transfer prerequisites remained truthful with this empty workspace.
- Ran 28 real page-width checks: Home, Tasks, Accounts, and Help at 320, 360, 390, 768, 1024, 1280, and 1440 CSS pixels. No page-level horizontal overflow was measured.
- Inspected the six Simple navigation destinations at 390px and saved screenshots. The guide no longer reproduced the old runtime's overflow.
- Simulated browser offline state and refreshed Tasks; captured the false all-clear before the fix and the honest unavailable state afterward.

## Phase-by-phase qualification

| Phase | Evidence from this smoke pass | Remaining qualification |
|---|---|---|
| 1 — Research/content/prototype | Compared real Simple wording and hierarchy against the plan | Technical copy remains in account creation, funding, and Help; user research acceptance is not complete |
| 2 — Technical blockers | Clean image builds, fresh PostgreSQL migration, real opaque sessions, Linux targeted race test | Full migration-compatibility/integration/race/retention qualification remains; shared production infrastructure intentionally unprovisioned |
| 3 — Design system/shell | Real mode persistence, navigation, mobile reflow, session lifecycle | Full manual assistive-technology and cross-browser coverage not established by Chromium smoke tests |
| 4 — Home/Tasks | Real empty and one-account Home, Tasks loading/offline fixes, incomplete-list warning | Tasks does not yet prove complete aggregation of every recovery/webhook/financial attention source |
| 5 — Core workflows | Real zero-balance account creation and empty-workspace prerequisites; fixture command regressions | Real funded transfer, approval, correction, and unknown-outcome lifecycle acceptance was not exercised in this pass |
| 6 — Expert/modularity | Expert routes load, relevant evidence remains available | Route availability does not prove complete hotspot decomposition or every advanced workflow |
| 7 — Verification/usability | Automated checks plus real browser inspection and screenshots | Five moderated operator sessions and unresolved repository gates remain |

## Automated checks

- Frontend lint: passed.
- Frontend unit/security/UI suite: 198 passed, zero skipped.
- Focused Tasks-truth and session-boundary Playwright regressions: 5 passed.
- Tasks desktop/mobile screenshot baselines: intentionally updated for the truthful incomplete-list notice and revised introduction; reviewed manually.
- Full Playwright suite: **215 passed in 1.2 minutes**, including accessibility, responsive, visual regression, financial failure-state, and performance tests. Run after fixes without blanket snapshot updates. The logged confidential-render probe is an intentional safety fixture, not an unexpected runtime error.
- Linux PostgreSQL integration: `go test -race -count=1 -v ./tests/integration -run '^TestOpaqueSessionsAndPreferencesOnPostgreSQL$'` passed against the isolated migrated database.
- Clean Linux production frontend image and API images: built; isolated services became healthy.
- Vercel configuration validator: passed; no deployment performed.
- Production npm audit: zero vulnerabilities. Full audit: **17 findings (2 critical, 5 high, 10 moderate)** in the development dependency tree. This gate is not clear.
- Generated developer-artifact check: **failed** (`contracts/generated/go/ledgersync.go` drift). The generator must respect internal-only contracts before artifacts are regenerated; blindly publishing BFF session contracts is not an acceptable fix.
- `git diff --check`: passed; Git reported existing LF/CRLF normalization warnings.

**Correction to earlier evidence:** `go test ./...` succeeding without configured integration dependencies does not prove live database tests ran. Harness-dependent tests may skip. Only the explicitly configured PostgreSQL test above is claimed as newly verified live database/race coverage here. Similarly, the zero-vulnerability result applies only to production dependencies, not the full npm tree.

## Screenshot evidence

- [Old runtime: Tasks missing](old-runtime-tasks-missing.png)
- [Old runtime: mobile guide overflow](old-runtime-guide-mobile.png)
- [Redesign: sign-in loop before cookie fix](redesign-login-loop.png)
- [Before: offline Tasks falsely all clear](tasks-offline-false-all-clear.png)
- [After: offline Tasks needs checking](tasks-offline-fixed.png)
- [Simple Home](simple-home-mobile.png)
- [Accounts](simple-accounts-mobile.png)
- [Add money](simple-funding-mobile.png)
- [Transfers](simple-transfers-mobile.png)
- [Tasks](simple-tasks-mobile.png)
- [Help](simple-guide-mobile.png)

## Next gates

1. Repair public generated-artifact filtering/drift and address or explicitly time-bound high/critical development-tool vulnerabilities.
2. Complete plain-language Simple workflows and the missing Tasks attention-source coverage.
3. Run the full real-backend financial lifecycle, migration compatibility, race, authorization, and retention gates in a disposable test environment.
4. Complete the five moderated user sessions and fix repeated confusion. No automated or AI inspection substitutes for that acceptance criterion.

The isolated stack is left available for local review on port 3300. The older port-3000 stack and its data remain intact. No production-readiness claim is made.
