# Guided Workspace — design QA

Status: **blocked — full redesign acceptance is not complete**.

Route-by-route status: `docs/design/qa/guided-workspace-completion-matrix.md`.

## Implemented preview

Local isolated production stack: `http://127.0.0.1:3300/welcome`. Anonymous `/` introduces the product; authenticated `/` opens operational Home. The normal port-3000 development stack was not replaced.

Implementation lives in the existing `codex/003-simple-first-hardening` integration worktree. Pre-existing edits were preserved. No deployment, cloud provisioning, secret-file change, commit, or push was performed.

## Visual comparison

The third selected image (`exec-b142f8ba-7a55-49f7-ae17-d7f61464624a.png`) and the implemented transfer review were opened together repeatedly at a 1487 × 1058 viewport. The implementation screenshot is full-page; financial fixture values differ from the illustrative concept intentionally.

Matched: light top navigation, navy/blue interface, three-step progression, prominent exact amount, vertical account journey, visible expected effects, pale-blue contextual guidance, and disclosed technical evidence.

Fixed after comparison: duplicated review heading, excessive nested padding, oversized step markers caused by legacy CSS, mobile preview overflow at tablet widths, unreadable full references on mobile, missing before/after balance comparison, and missing source freshness. The skip link is verified off-screen until focused; screenshot capture now resets scrolling after viewport changes.

Intentional differences: the 320px help rail and 34px application heading follow the written plan rather than the larger illustration. Instructional checkmarks were not copied because they could falsely imply completed verification. Financial warnings remain in the main column. Confirmation does not claim completion in advance.

Evidence:

- `docs/design/qa/guided-transfer-review-desktop.png`
- `docs/design/qa/guided-landing-desktop.png`
- `docs/design/qa/guided-landing-mobile.png`

The landing page was also inspected in the in-app browser, including its actual sign-in transition. Real Home, Tasks, and Corrections were inspected against the isolated API. A real empty-corrections serialization defect was found, repaired, and rechecked in the browser.

After the integration suites reset their disposable fixtures, the existing empty-workspace bootstrap was restored (tenant, roles, and local policies only). The final landing → sign-in → empty operational Home transition was rechecked successfully. The disposable earlier smoke-test account was reset by the test harness; normal port-3000 workspace data was not touched.

## Verification recorded on 2026-09-05

- Frontend unit/security/UI: **206 passed**.
- Browser behavioral/scenario checks: **223 passed** with `--ignore-snapshots`. This is explicitly **not** a visual-regression pass. No checked-in screenshot tolerance was loosened and no failing test was quarantined.
- Frontend lint and TypeScript: passed.
- Production Next.js build and clean Linux `npm ci`/container build: passed.
- Public OpenAPI validation, generated-artifact currency check, Go contract tests, and Vercel configuration validation: passed. Internal session operations are excluded from generated public SDKs and Postman collections.
- npm dependency audit: zero vulnerabilities after upgrading the pinned validator/YAML tooling and repairing Linux optional dependencies in the lockfile.
- Performance budget: 1,741,590 bytes total JavaScript, 229,156-byte largest chunk, and one 48,256-byte font at the measured build; all below the supplied limits.
- Standard `go test ./...`: exits successfully, but dependency-backed skips are **not** qualification passes.
- Disposable PostgreSQL opaque-session/preferences race test: passed.
- Full disposable PostgreSQL/Redis integration and fault suites: **passed with `-race -p 1 -count=1`**, with no skips in that run. Package serialization is necessary because the suites reset shared disposable fixtures. Migration compatibility now explicitly requires migration 37, its session/preferences tables and indexes, and the corresponding database-derived diagnostic schema version.

A final additional `npm audit --omit=dev` was blocked by the approval reviewer because it transmits dependency metadata to the npm registry. No workaround was used. The earlier clean Linux install audit reported zero vulnerabilities; the blocked command is not counted as a new audit result. Rerunning it requires explicit approval for that metadata upload.

## Remaining acceptance blockers

1. Funding, approval, correction, lifecycle, reconciliation, and replay workflows still need the complete new guided presentation and corresponding controller/storage review. Existing functional coverage is not proof that those redesign phases are delivered.
2. Expert-page decomposition, dead CSS/navigation cleanup, global localized-money adoption, and exhaustive unavailable-action/copy migration remain unfinished.
3. Home and Tasks need the remaining cross-source deduplication, complete recommended-action integration, and source-specific recovery/actionability refinements. A bounded loaded page must never imply globally complete coverage.
4. Screenshot baselines still represent the previous presentation. The full visual gate remains open until every changed baseline is manually reviewed. Do not promote the snapshot-disabled run to an acceptance pass.
5. Full real-browser funding/approval/correction/response-loss qualification, all-package race/system and operational restore qualification, and production-container security qualification remain open beyond the tests explicitly recorded above.
6. Five moderated operator sessions have not occurred. No usability scores or participant findings are invented.

The safe next delivery slice is the remaining core guided workflows, starting with persistent, exact funding-request recovery. This document must not be read as production approval or as completion of all seven phases.
