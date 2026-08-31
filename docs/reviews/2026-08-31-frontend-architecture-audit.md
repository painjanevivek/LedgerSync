# LedgerSync frontend architecture, UX, and completion audit

**Date:** 2026-08-31

**Scope:** `web/`, `dashboard/`, frontend-facing BFF routes and security boundaries, UI plans/evidence, recent frontend history

**Authority order:** repository code and tests → `docs/plans/ledgersync-master-progress.md` → current master completion plan → older plans
**Current master truth:** M07 active; M08 and M09 partial; local-only qualification does not imply production approval

## Method and evidence boundary

This review read the requested repository sources, including `README.md`, `DESIGN.md`, `design-qa.md`, the master register and completion plan, future-scope and cleanup plans, `web/AGENTS.md`, application routes, feature and library modules, styles, tests, both frontend manifests, the archived dashboard, supported/legacy deployment boundaries, and recent frontend commits. Recent commits from `4b43173` through `7eb1a1d` show active UX work in `web/`; `dashboard/` has not changed since the initial repository import (`c67412a`).

The repository-memory index was stale. A requested re-index was not performed because the indexing service explicitly reported that private repository source would leave the local environment. This audit therefore uses direct local repository and Git evidence. No remote repository state, provider state, production approval, physical-device result, or external review is inferred.

Qualification performed on the resulting working tree:

- `npm --prefix web run lint`: pass.
- `npm --prefix web run test`: 100/100 pass, including security and UI contracts.
- `npm --prefix web run build`: pass on Next.js 16.3.2, including TypeScript and all App Router/BFF routes.
- `npm --prefix web run test:e2e`: 136/136 pass, including mocked workflows, axe, responsive/reflow, forced-colors, reduced-motion, keyboard, and visual baselines.
- `npm --prefix web run test:e2e:performance`: 2/2 pass; compact constrained-4G evidence was LCP 1452 ms, INP 24 ms, CLS 0, 32 initial requests, one 85 ms long task.
- `npm --prefix web run test:performance`: pass; 1,021,254 total JavaScript bytes, 229,156-byte largest chunk, no bundled font files, all below repository budgets.
- Real-stack browser qualification was not run: it requires an explicitly approved isolated Compose project, mutation opt-in, a seeded account, and a unique run ID (`web/tests/system/README.md`). A dirty audit tree is also not exact-commit release evidence.
- Physical device, real browser zoom, NVDA, VoiceOver, provider, partner, legal, penetration, managed AWS, and production gates remain external/manual evidence.

## 1. Executive Verdict

LedgerSync has a mature, high-integrity **local operator console**, not a production-complete frontend. Its strongest characteristic is financial truthfulness: exact string/minor-unit money, separate financial and delivery outcomes, durable idempotent intent, fail-closed authorization, explicit unavailable/offline/unknown states, reconciliation evidence, and bounded BFF responses are embedded in both code and unusually strong tests. The UX has progressed beyond a developer demo: onboarding, evidence freshness, deep links, responsive behavior, accessible dialogs, exports, recovery, and developer surfaces are real.

The main risk is concentrated architecture debt rather than broad low quality. `OperatorConsole` is a cross-domain client controller; session/network/sign-out orchestration is repeated across six consoles; navigation is not role/environment aware; list standards are uneven; high-risk funding/correction posting lacks the confirmation pattern already used by account lifecycle; shared primitives and CSS exist but remain accumulated in a client-only component module and a 5,103-line global stylesheet. Browser observability is correlation-reference based, not a complete sanitized frontend telemetry system. The archived `dashboard/` is still tracked and creates avoidable product ambiguity.

Technical debt is **moderate overall and high in four hotspots**: controller boundaries, IA, CSS/primitives, and legacy removal. UX maturity is **strong for current local workflows, partial for cross-domain investigation and administration**. Production readiness is low because M11–M17 and multiple external gates are intentionally incomplete, not because the local web app is generally broken.

### Scorecard

| Area | Score | Evidence-based reason |
|---|---:|---|
| Architecture | 7/10 | Clear App Router/BFF/domain layering and server-only private API modules; weakened by client-heavy route controllers and duplicated console lifecycle code. |
| Code quality | 8/10 | Strict TypeScript, bounded parsers, explicit states, exact-money helpers, no unsafe financial optimism, strong names and tests; several compressed/oversized modules reduce reviewability. |
| Maintainability | 6/10 | Feature folders and shared primitives help, but `OperatorConsole` (736 lines), `CorrectionsConsole` (833), `TransferViews` (568), `ConsoleShell` (391), and 5,103 global CSS lines create coupled change surfaces. |
| UX | 8/10 | Deliberate operator language, onboarding, evidence-first details, safe retries, exports, and empty/error distinctions; IA and a few high-risk confirmation paths remain incomplete. |
| Accessibility | 8/10 | Automated axe, reflow, text spacing, keyboard, focus, forced-color, reduced-motion, touch-target, and exact-identifier coverage are excellent; screen-reader/manual and several dense-table/timeline tasks remain. |
| Responsive design | 9/10 | 320–2560 px matrices, zoom-equivalent reflow, compact modal navigation, landscape/rotation evidence, and reviewed baselines pass; duplicated breakpoints and compressed overrides are maintainability risks. |
| Performance | 8/10 | Measured web-vital, request, long-task, JS, font, and large-list budgets pass; the client boundary remains wider than necessary and CSS is large. |
| Test quality | 9/10 | 100 unit/security plus 136 browser cases, visual baselines and performance budgets; real-stack and physical/manual evidence are intentionally separate. |
| Developer experience | 8/10 | Simple dependencies, deterministic scripts, versioned contract/download surface, strong test fixtures and local runbooks; large files and duplicated controllers slow safe iteration. |
| Operational UX | 7/10 | Operators can trace accounts, transfers, events, reconciliation, corrections, funding and exports; no unified investigation workspace, approval inbox, webhook workspace, or consistent list contract yet. |
| Production readiness | 4/10 | Local-only product is qualified; managed identity/tenancy, infrastructure, observability, compliance, DR, pilots and release authority remain partial/pending/external in M11–M17. |

**Frontend-completion status: PARTIALLY COMPLETE.** M08/M09 work and accepted P1 architecture/UX work remain, the legacy surface is unresolved, and external/manual gates are not complete.

## 2. Current Frontend Architecture Map

### Runtime and route flow

```text
App Router page (Server Component by default)
  /, /accounts/**, /transfers/**, /reconciliation/**, /guide
    → OperatorConsole client controller
      → useAccountWorkspace / useTransferSubmission / useReconciliationCommand
      → feature views (Overview, Account, Transfer, Reconciliation, Orientation)

  /funding/**      → FundingConsole → FundingRequestFlow / FundingViews
  /corrections/**  → CorrectionsConsole → list/detail/action UI
  /events/**,
  /local-status    → OperationsConsole → EventViews / LocalStatusView
  /developer       → DeveloperConsole → dynamically loaded DeveloperView
  /recovery        → RecoveryConsole → dynamically loaded RecoveryView
  /admin           → deny-by-default notFound()

Client read/mutation
  → lib/api/client.ts or domain command hook
  → same-origin /api/** route
  → signed HttpOnly session + Host/origin/CSRF/scope/idempotency checks
  → privateAPIContext()
  → workload bearer + short-lived actor assertion + request ID
  → private LedgerSync API
  → PostgreSQL-authoritative ledger / Redis-disposable projections
```

Thin route files correctly await Next.js 16 async `params`/`searchParams` and pass bounded identifiers/filter state to controllers (`web/src/app/**/page.tsx`). Root layout is forced dynamic so the proxy-provided CSP nonce is per request (`web/src/app/layout.tsx:5-7`). `web/src/proxy.ts` owns Host validation and CSP/security headers.

### State and data flow

```text
route params/query
  → controller props
  → local React state + domain hooks
  → readJSON(cache: no-store, X-Request-ID, 8s timeout)
  → explicit loading / ready-empty / populated / stale / unavailable /
    forbidden / offline / unknown-after-submit UI

financial command
  → canonical exact intent
  → tenant/object-scoped sessionStorage retry record
  → immediate in-flight lock
  → CSRF + Idempotency-Key + bounded JSON BFF request
  → authoritative response, typed final rejection, or unknown outcome
  → same-key/same-body retry only; never optimistic money movement
```

`web/src/lib/api/client.ts:3-13` defines the canonical UI state vocabulary. Reads are `no-store` with timeouts (`:26-49`). `web/src/features/transfers/useTransferSubmission.ts:47-188` binds retained transfer intent, in-flight locking, request references, and unknown outcomes. `web/src/app/api/transfers/route.ts:10-74` preserves BFF authorization, CSRF, idempotency, private timeout classification, no-store, and signed consistency requirements.

### Shared UI, style and test architecture

```text
DESIGN.md visual/interaction contract
  → styles/tokens.css
  → app/globals.css + styles/responsive.css
  → features/console/components.tsx
     FormField, CopyControl, StatusBadge, StatePanel, FocusedRetry,
     RecordLink, Pagination, DataTableRegion, PageHeader,
     EvidenceFreshness, EvidenceStepMarker
  → domain views and action workflows

Node test runner
  → security boundary + domain/state + UI contract tests
Playwright mocked production build
  → journeys + accessibility + responsive + visual + performance
Playwright isolated real stack
  → browser → BFF → API → PostgreSQL/Redis, mutation opt-in only
```

## 3. Canonical Frontend Decision

| Surface | Decision | Exact evidence | Required action |
|---|---|---|---|
| `web/` | **KEEP — canonical** | Next 16/React 19 (`web/package.json:26-27`); all supported App Router/BFF routes; CSP/session/private API boundaries; 100 passing unit/security tests; 136 passing browser tests; current UX commits; supported Compose/deploy references. | Continue M08/M09 here. Document controller/UI/CSS target architecture and preserve qualification gates. |
| `dashboard/` | **REMOVE as one coordinated archived slice** | Next 14/React 18 (`dashboard/package.json:12-13`); only runtime relation is `docker-compose.legacy-demo.yml`; supported production boundary explicitly forbids `dashboard/` (`tests/contract/production_boundary_contract_test.go:33-35`); its three Python tests are placeholders (`tests/dashboard_test.py:24,29,34`); Git history stops at initial import. The cleanup audit gives 98% confidence to remove it with `backend/`, `simulation/`, `setup/`, legacy Compose, and legacy tests (`docs/reviews/2026-08-25-phase-11-cleanup-audit.md:13,34`). | Do **not** migrate components into `web/`. Delete only the entire D-01 legacy slice, update current README/threat-model wording, invert the boundary contract to assert absence, run root/web/Compose/security qualification, and preserve historical audit documents. |

Why both exist: `dashboard/` is an archived predecessor/demo; `web/` is the actively evolved operator console and BFF. Retaining the predecessor as executable-looking source no longer provides migration value and increases dependency, security-scanner, onboarding and “which UI is real?” ambiguity. Removing `dashboard/` alone would leave a misleading partial legacy topology; the existing coordinated cleanup boundary is correct.

## 4. What Is Already Good

- **Financial invariants are visible in UI architecture.** Exact minor-unit strings and explicit currency are centralized in `web/src/lib/money.ts`; overview refuses mixed-currency aggregation and separates customer funds (`OverviewView.tsx:46-49,81-91`).
- **Unknown outcomes are first-class.** Transfer and reconciliation commands persist exact intent and retry identity; final rejection and unknown timeout are distinct. There is no optimistic balance movement.
- **Authorization is server-owned.** Browser-visible scopes gate affordances, while BFF routes independently validate signed session, Host/origin, CSRF, method, payload, rate and scope. UI hiding is not treated as security.
- **The BFF does not expose backend credentials.** `private-api.ts:11-22` uses a workload credential and actor assertion server-side; `server-only` prevents client import.
- **Session handling is strong.** HMAC signatures and timing-safe comparison (`session.ts:24-45`), bounded roles/scopes/consistency requirements, HttpOnly/SameSite cookies and production-secure behavior (`:78-92`) are appropriate.
- **Truthful UI states are unusually mature.** `unavailableMessage` explicitly says prior evidence is historical and no empty/success state is inferred. Views preserve timestamped evidence on refresh failure.
- **Account data races are mostly treated seriously.** `useAccountWorkspace` uses separate directory/detail/balance/history generations (`:53-114`) and URL-preserved filters/return context.
- **Mutation double-submit protection exists.** Immediate ref locks and disabled controls are covered by unit/browser tests; idempotency keys are bound to canonical bodies.
- **Correction replaces mutation.** Original journals remain immutable; reverse transfer/correction evidence is paired and approval separation is enforced.
- **Operational evidence is cross-linked.** Events link to transfers/accounts; reconciliation mismatches link to affected accounts; export dialogs disclose scope/filters/columns; transfer detail exposes stored evidence stages.
- **The visual direction is coherent.** Dark navigation, restrained status colors, document/docket metaphors, dense but deliberate evidence, tabular numbers, and explicit UTC are consistent with `DESIGN.md`.
- **Responsive/accessibility engineering is real.** Native dialog semantics, focus trapping/restoration, inert compact navigation, visible focus, touch targets, sticky table identifiers, horizontal table regions, reduced motion, forced colors and long-ID handling are tested.
- **Dependencies are disciplined.** No state/query/form/UI framework is missing by default. React, Next, jose and Phosphor are sufficient for the demonstrated current architecture.
- **Optional heavy views are already split.** Developer and Recovery views use `next/dynamic` (`DeveloperConsole.tsx:14`, `RecoveryConsole.tsx:15`).
- **Performance is enforced numerically.** JS/font budgets, web vitals, request counts, API counts, long tasks and large-list responsiveness are repository gates, not subjective claims.
- **The test suite mirrors the product's risk.** Security, exact money, idempotency, authorization, unknown outcomes, reflow, accessibility, visual states and real-stack mutation boundaries have dedicated evidence.

These are foundations to preserve. Replacing them with generic client caches, optimistic mutation UI, a broad state store, or a cosmetic component-kit rewrite would add risk without solving a demonstrated problem.

## 5. Current Problems and Required Changes

| ID | Priority | Problem | Evidence / where | Why it matters | Recommended change / how | Owner | Effort | Dependency | Acceptance criteria |
|---|---|---|---|---|---|---|---|---|---|
| F01 | P1 | Archived frontend remains tracked | `dashboard/package.json`; `tests/dashboard_test.py:24-34`; cleanup audit D-01; production boundary contract | Confuses canonical ownership, retains obsolete dependencies and increases scanner/onboarding surface. | Execute the existing coordinated legacy-slice removal; do not migrate it. | Staff Engineer + Backend + Release | M | Clean-tree coordinated change | No legacy slice; boundary test asserts absence; root/web/Compose/security gates pass. |
| F02 | P1 | `OperatorConsole` is a cross-domain god controller | `OperatorConsole.tsx:75-123` holds 20+ state slots; `:127-317` loads transfers/runs/orientation/explainability; `:353-437` owns session and initial orchestration; `:500-735` routes views | A change to one domain can race or regress another; review and test scope is too broad. | Split route-specific controllers/hooks behind a shared console session boundary. Add generation/single-flight protection to transfers and reconciliation before extraction. | Senior/Staff Frontend | L | F03 | No controller owns more than one domain request graph; retained-state tests pass; no extra initial/API requests. |
| F03 | P1 | Session, online and sign-out lifecycle is repeated six times | `/api/session` in Operator, Funding, Corrections, Operations, Developer, Recovery consoles; `navigator.onLine` and sign-out repeated in the same modules | Inconsistent error/focus/loading changes and duplicate implementation cost. | Create a narrow `ConsoleSessionProvider`/`useConsoleSession` client island with session, online state and sign-out. Do not cache financial records in it. | Frontend | M | Performance request budget | One session request per route load, all scope-denial states preserved, no provider above static root unnecessarily. |
| F04 | P1 | Navigation is static rather than role/environment aware | `ConsoleShell.tsx:54-125` always renders Financial workspace and “Local tools”; master plan requires Local Status only in local mode and Administration only for authorized production users (`master...plan.md:612-624`) | Production users may see irrelevant local tooling; IA cannot express approvals/webhooks/admin ownership. | Pass a server/session-derived navigation capability model; group Work, Investigate, Platform; keep related tools contextual. | Product + Staff Frontend + Security | M | M08 routes/policies | Local Status absent outside local; unauthorized sections not linked; direct routes still fail closed; automated nav matrix passes. |
| F05 | P1 | M08 list/detail contract is uneven | Master requirements at `master...plan.md:626-655`; Accounts/Events have URL filters, but funding/corrections lack complete search/date/sort/result-count URL state; no list exposes user sorting | Operators lose investigation context and cannot share/reproduce every view. | Define a server-owned `ListQuery` contract per domain and a common list header/result summary; add capabilities only where backend allowlists exist. | Frontend + Backend + Product | L | API query contracts | Each M08 list documents default sort, query parameters, count semantics, cursor context, export parity, clear-all and back context. |
| F06 | P1 | CSS architecture is accumulated and breakpoint logic is duplicated | `globals.css` 5,103 lines/106 KB; funding begins near `:4126`, corrections `:4831`; `responsive.css` repeats 760 and 520 blocks (`:41,84,195,218`) and uses compressed multi-selector lines; hard-coded colors remain (`globals.css:112,1023,1559`; `responsive.css:50,81`) | Cascade ownership is difficult to review; feature changes risk distant regressions. | Introduce explicit ordered layers/files and migrate one feature at a time without visual redesign. Replace repeated literals with semantic tokens after visual tests. | Frontend + Design | L | F07 primitive ownership | No specificity increase; feature styles have owners; duplicate breakpoints reduced; all 136 browser/visual tests and CSS contracts pass. |
| F07 | P1 | Shared primitives are useful but client-only and incomplete | `features/console/components.tsx:1` is `use client`; hook-free `StatusBadge`, `StatePanel`, `PageHeader`, `DataTableRegion` share its client graph; buttons/dialogs/money/identifier/timestamp patterns remain class/copy based | Stable read-only UI sends avoidable JS and recurring high-risk patterns can drift. | Split server-compatible display primitives from interactive controls under `src/ui/`; add only primitives with recurring consumers. | Staff Frontend + Design + A11y | L | CSS migration | Server components can import static primitives; interactive boundaries remain leaf-level; bundle/request budgets do not regress. |
| F08 | P1 | High-risk post actions are less guarded than account lifecycle | Funding “Post balanced journal” is a direct click (`FundingViews.tsx:168`); correction “Post exact reverse transfer” is a direct click (`CorrectionsConsole.tsx:795`); account lifecycle uses an evidence-refreshing modal | An accidental activation can create an immutable financial movement even though backend correctness holds. | Reuse a `ConfirmationDialog` composition that refreshes authoritative record status/version, states exact effect, requires a reason/explicit confirmation where policy needs it, and preserves idempotency. | Frontend + Financial + A11y | M | Backend precondition/version contract | Keyboard-modal, focus restored, stale conflict fails closed, exact effect displayed, duplicate activation one request, unknown retry same body/key. |
| F09 | P1 | Route render and 404 fallbacks are framework-generic | No tracked `error.tsx`, `global-error.tsx`, or `not-found.tsx`; `/admin` calls `notFound()` (`app/admin/page.tsx:5`) | Unexpected rendering failures do not use LedgerSync's no-inference language; generic 404 is less deliberate. | Add generic non-disclosing route/404 boundaries **only with an offsetting two-request reduction**. Audit experiment showed naïve root files increase initial scripts from 32 to 34 and fail the performance gate; the files were reverted. | Staff Frontend + Security | M | F03/F07 bundle reduction | No error message/digest/tenant disclosure; retry works; admin stays non-disclosing; initial requests ≤32 and all gates pass. |
| F10 | P1 | Some async loaders lack generation/single-flight protection | `OperatorConsole.loadTransfers` at `:127-158` and detail/run loaders write after await without generation; `useAccountWorkspace.loadMoreHistory` at `:151-160` lacks generation/in-flight; Funding was corrected in this audit | Rapid filters, route changes or double pagination can append stale/duplicate evidence. Financial source truth is unchanged, but displayed investigation evidence can be misleading. | Add request generation keyed by query/object plus immediate pagination in-flight refs; preserve prior verified evidence and timestamps. | Frontend | M | F02 | Deterministic race tests prove older responses cannot overwrite/append and rapid load-more dispatches once. |
| F11 | P1 | Browser observability stops at safe request references | Correlation IDs are propagated throughout `lib/api/client.ts` and views; repository search finds no web instrumentation/error/vital reporting module | Operators can copy a reference, but SRE cannot measure client render faults, workflow abandonment or real-user performance in a managed environment. | Define a sanitized telemetry schema: route template, state transition, performance metric, error class and request reference only; forbid IDs, amounts, free text, CSRF, payloads and query content. | SRE + Security + Frontend | M | M12/M13 managed collector | Reviewed allowlist, consent/retention decision, redaction tests, CSP-compatible transport, alert/SLO ownership. |
| F12 | P2 | Funding account picker previously collapsed failures/partial scope | Before this audit `FundingConsole.loadAccounts` ignored non-OK and `next_cursor`, while `FundingRequestFlow` rendered the resulting array | An unavailable or >100-account directory could appear as no/partial eligible accounts. | **Implemented:** explicit loading/error/scope completeness, generation guard, focused retry, and fail-closed entry. | Frontend | Complete | None | Unit contracts pass; no select rendered for unavailable/incomplete scope; build/browser/performance pass. |
| F13 | P1 | Account conflict result focus had a race | `AccountLifecycleActions` closed the dialog, scheduled trigger restoration, then scheduled outcome focus; full E2E exposed nondeterminism | Keyboard/screen-reader users could miss the reason a financial lifecycle command did not complete. | **Implemented:** explicit outcome-focus ownership suppresses trigger restoration on conflict. | Frontend + A11y | Complete | None | Full account conflict E2E passes and heading owns focus. |
| F14 | P2 | Funding request panel is visually modal-like but semantically inline | `FundingRequestFlow.tsx:82-101` renders a section with close button; no focus entry/restoration or dialog behavior | Keyboard users may not discover the newly opened workflow; background remains in tab order though the panel dominates attention. | Decide intentionally: either keep inline and focus the heading on open with return focus, or use native dialog and the shared confirmation/dialog contract. | Frontend + Design + A11y | S | F07 | Open announces heading; close returns focus; Escape behavior matches chosen pattern; axe/keyboard tests. |
| F15 | P2 | Large tables are bounded but lack a scaling policy beyond cursor pages | Accounts/transfers/events/reconciliation use 25-row cursor pages and scroll regions; selectors can request 100 accounts; no virtualization, column priority control or server sort | Current sizes are safe, but operational investigation at larger tenant volumes will require more precise server-side narrowing rather than larger payloads. | Prefer server filtering/sort/cursor and saved views; virtualize only after measured DOM/render pressure, never to mask unbounded API reads. | Frontend + Backend | M | F05, scale evidence | 10k-record backend dataset still sends bounded pages; interaction long-task ≤250 ms; accessible row/column context retained. |
| F16 | P2 | Documentation phase truth had drifted | README claimed Phase 2 active while master register says M07 (`README.md:7`, register `:7-9`) | Contributors can execute the wrong roadmap. | **Implemented:** README now points to M07 active and M08/M09 partial; register evidence refined without claiming completion. | Product/Engineering | Complete | None | README and master register agree. |

## 6. Component and Architecture Refactor Map

Refactoring is justified by mixed responsibility and change coupling, not line count alone.

### Operator console — refactor required

```text
CURRENT
App page → OperatorConsole
  → session/network/sign-out
  → account workspace
  → transfer list/detail/explainability
  → reconciliation list/detail
  → local orientation/preferences
  → every account/overview/transfer/reconciliation/guide view

PROPOSED
App page
  → ConsoleSessionBoundary (session + online + sign-out only)
  → route-specific FeatureController
      OverviewController
        → useOverviewEvidence (parallel read-only composition)
        → OverviewView
      AccountsController
        → useAccountWorkspace
        → directory/detail/create/lifecycle views
      TransfersController
        → useTransferEvidence + useTransferSubmission
        → TransferListView / TransferDetailView / TransferForm
      ReconciliationController
        → useReconciliationEvidence + useReconciliationCommand
        → ReconciliationView
      GuideController
        → orientation evidence/preferences only
  → server-compatible UI primitives + interactive leaf controls
```

The controller boundary owns route/query interpretation, request generations and view-model composition. Domain hooks own only their API state machine. Views receive immutable view models and callbacks. The session boundary must not become a global financial cache.

### Corrections console — cohesive domain, mixed layers

`CorrectionsConsole` is not a false abstraction merely because it is 833 lines: it contains one correction domain and its separation-of-duties rules. It nevertheless mixes session orchestration, list/detail fetching, mutation transport, status policy and two large presentational trees.

```text
CorrectionsConsole
  → CorrectionsController (route/session/query wiring)
  → useCorrectionEvidence (list/detail generation + freshness)
  → useCorrectionCommand (approve/reject/cancel/post intent and outcomes)
  → CorrectionListView
  → CorrectionDetailView
  → CorrectionDecisionPanel
  → shared ConfirmationDialog / StatePanel / evidence primitives
```

Do not move backend approval policy into the view. The hook may interpret typed outcomes; the BFF/backend remain authoritative.

### Funding — partial boundary already exists

```text
FundingConsole
  → useFundingEvidence (records, detail, reconciliation, eligible account scope)
  → useFundingCommand (approve/reject/post/compensate)
  → FundingListView / FundingDetailView / FundingWorkspaceRail
  → FundingRequestFlow
```

`FundingRequestFlow` is a legitimate two-step cohesive form and should remain separate. The audit's fail-closed account-scope change belongs in the evidence controller. Move direct posting into the shared guarded-command pattern before deeper cosmetic decomposition.

### Transfer modules — split views, keep the state machine

`useTransferSubmission` is cohesive and should stay a dedicated audited state machine. `TransferForm` is also reasonably cohesive: it validates exact input, prepares review and delegates submission. `TransferViews` contains list, detail, postings, evidence timeline and related-record presentations and should split by route/presentation:

```text
TransferViews.tsx
  → TransferListView.tsx
  → TransferDetailView.tsx
  → TransferEvidenceTimeline.tsx
  → PostingTable.tsx
  → TransferOutcomePanel.tsx
```

Share domain types/formatters, not a generic “financial card” abstraction.

### Reconciliation — keep command cohesive; separate evidence transport

`ReconciliationCommand` (259 lines) is a coherent long-running command with review, same-key retry, polling deadline, active-run semantics and result presentation. Keep the command state machine together. Extract only the result/review presentations after a `useReconciliationCommand` API is stable. Add generation/single-flight protection to history/detail loading in the route controller.

### Operations/events — split list and detail files

`OperationsConsole` is small enough but owns two resources with different schemas. Retain one controller only if its request-generation helper is shared; split `EventViews.tsx` into list, detail and timeline modules. `LocalStatusView` remains independent. Event URL filters and linked evidence are good and should be the M08 reference implementation.

### Console shell — split static frame from compact behavior

`ConsoleShell` legitimately needs client behavior for `matchMedia`, native-dialog-like drawer focus, Escape, `inert`, and focus return (`:146-333`). The visual frame, brand, static navigation records and tenant metadata do not inherently require hooks.

```text
ConsoleShell (client everything)
  → ConsoleFrame (server-compatible structure/slots)
  → ConsoleNavigation (capability-filtered static links)
  → CompactNavigationController (client matchMedia/focus/inert)
  → OperatorIdentityMenu (client sign-out)
```

Do not replace the semantic `matchMedia` check with CSS alone: JavaScript is required because compact mode changes focus containment and dialog semantics. Centralize the `760px` semantic breakpoint so CSS and JS cannot drift.

### Overview, Recovery and Developer

- `OverviewView` is a good presentational composition. Keep independent account/transfer/reconciliation state panels. Longer-term, server-render stable headings/empty shells and isolate LocalOrientation/action controls, but do not create a server fetch that bypasses BFF consistency.
- Recovery and Developer dynamic imports are appropriate optional-route splitting. Their controller files are compressed and repeat session lifecycle; migrate them to F03, but do not merge their distinct domain views.

## 7. Design-System / UI Primitive Plan

Target location: `web/src/ui/` with a public entrypoint for server-compatible display primitives and separate explicit client entrypoints. Avoid a package/monorepo until the separate public site (M10) proves cross-product consumption.

| Existing/repeated pattern | Decision | Proposed location | Consumers | Tests required |
|---|---|---|---|---|
| `FormField` | Keep, improve error association API | `ui/forms/FormField.tsx` (client only if `useId` remains necessary) | Every form | Required/optional, hint/error IDs, invalid focus summary, text spacing. |
| `StatusBadge` | Keep; server-compatible | `ui/status/StatusBadge.tsx` | All lists/details | Color-independent icon/text, forced colors, vocabulary snapshot. |
| `StatePanel` | Keep; refine live-region policy so static empty/denied states are not always announced | `ui/state/StatePanel.tsx` | All domains | Static vs dynamic announcement, error alert, no inference copy. |
| `EvidenceFreshness` | Keep as core LedgerSync composition | `ui/evidence/EvidenceFreshness.tsx` | Overview, lists/details | UTC output, historical/refreshing/current, screen-reader text. |
| `CopyControl` | Keep interactive | `ui/controls/CopyControl.client.tsx` | IDs, references, commands | Clipboard success/failure announcement, full value accessible, secret ban. |
| `PageHeader` | Keep; server-compatible | `ui/layout/PageHeader.tsx` | Every route | Heading contract and responsive action wrapping. |
| `DataTableRegion` | Improve into composition, not a generic data engine | `ui/table/DataTableRegion.tsx` | Accounts, transfers, events, reconciliation, funding | Caption/region name, scroll hint, forced colors, sticky first column, optional sort semantics. |
| `Pagination` | Keep; add cursor result summary contract | `ui/navigation/Pagination.tsx` | Bounded lists/history | Single-flight activation, end-of-evidence copy, focus after append. |
| `.button` / `.icon-button` classes | Add typed wrapper gradually; do not rewrite all at once | `ui/controls/Button.tsx`, `IconButton.tsx` | Repeated guarded actions | Variant/size/disabled/busy, 44px targets, forced colors. |
| Native dialogs in lifecycle/export | Extract recurring focus and confirmation composition | `ui/overlays/ConfirmationDialog.client.tsx` | Account, transfer review, export, funding post, correction post | Modal semantics, Escape, initial/return/outcome focus, stale evidence conflict. |
| `formatMinorUnits` + repeated amount markup | Add display primitive; keep math in `lib/money.ts` | `ui/evidence/Money.tsx` | Overview, account, transfer, funding, reconciliation | Exact huge/signed values, explicit currency, tabular numerals, no floats. |
| Repeated `<code>` + CopyControl IDs | Add `Identifier` composition | `ui/evidence/Identifier.tsx` | All immutable IDs | Full accessible name, visual truncation only, copy behavior. |
| Repeated UTC helpers | Add `Timestamp` display primitive | `ui/evidence/Timestamp.tsx` | All evidence routes | Invalid/missing state, explicit UTC, `dateTime` value. |
| Inline notices and permission notes | Consolidate only semantic variants | `ui/state/InlineAlert.tsx` | Funding/corrections/permissions/offline | Icon+text, alert/status selection, forced colors. |
| Skeleton | **Not recommended now** | None | None | Existing explicit loading copy better preserves “unknown, not zero/empty.” Add only if measured layout stability needs it and semantics remain explicit. |

Primitives must encode LedgerSync semantics, not erase domain language. `Money` may format; it must never convert floats. `ConfirmationDialog` may orchestrate focus; it must never decide authorization or mutation policy. `DataTableRegion` may provide accessibility and layout; backend query contracts own sorting/filtering.

## 8. CSS Architecture Plan

### Current problems

- `tokens.css` is a strong 93-line foundation, but semantic values are bypassed by repeated literals.
- `globals.css` combines reset, shell, onboarding, account, transfer, reconciliation, recovery, operations, orientation, funding and corrections over 5,103 lines. Feature blocks were appended chronologically rather than owned structurally.
- `responsive.css` is only 220 lines but is densely compressed, repeats 760/520 media blocks, and mixes all feature overrides. The compact navigation breakpoint is also hard-coded in `ConsoleShell`.
- Multiple forced-color blocks appear throughout `globals.css` (`:2642`, `3061`, `3519`, `4103`, `4811`, `5091`), making coverage hard to reason about.
- Existing visual tests reduce migration risk but do not make cascade ownership maintainable.

### Target structure

```text
src/styles/
  tokens.css                 semantic color/type/space/target/z-index tokens
  foundations/
    reset.css
    document.css             body, typography, links, focus, selection
  layout/
    console-frame.css
    operator-workspace.css
    responsive-shell.css
  primitives/
    buttons.css
    forms.css
    states.css
    tables.css
    dialogs.css
    evidence.css
  patterns/
    record-list.css
    detail-document.css
    evidence-timeline.css
    guarded-command.css
  features/
    accounts.css
    transfers.css
    funding.css
    corrections.css
    reconciliation.css
    operations.css
    recovery.css
    developer.css
    orientation.css
  utilities/
    accessibility.css        sr-only, reduced motion, forced-color baseline
```

Use a declared cascade order (native `@layer` after a compatibility spike) or stable import order from root layout. Do not change CSS technology during extraction. New isolated components may use CSS Modules if they benefit, but existing global classes should migrate without mass renaming.

### Migration sequence

1. Freeze visual baselines and record computed-style contracts for buttons, fields, states, tables, dialog and shell.
2. Extract foundation/shell rules first with byte-for-byte declarations and unchanged import order.
3. Extract shared primitives used by three or more domains.
4. Move one contiguous feature block at a time: Funding (`globals.css:4126-4736`), Corrections (`:4831+`), Operations (`:2662-3060`), then account/transfer/reconciliation/recovery.
5. Co-locate each feature's responsive overrides with that feature or in one feature-responsive block; keep only shell breakpoints global.
6. Replace literals with semantic tokens only after extraction. Add tokens such as `--status-success-border`, `--nav-active-compact`, `--table-selected`, not raw palette aliases.
7. Consolidate forced-color and reduced-motion baselines; retain feature exceptions beside the feature.
8. After each batch run lint, unit UI contracts, affected visual specs, full phase-9 matrix and performance budgets.

Styles that remain global: reset, document/body, token definitions, fonts, global focus, shell grid, utility accessibility rules, native element baseline. Styles that become feature-owned: `.funding-*`, `.correction-*`, `.event-*`, `.recovery-*`, `.orientation-*`, domain action/evidence documents. Shared patterns move only when at least three consumers have the same semantics, not merely similar borders.

## 9. Server vs Client Optimization

There are 34 `"use client"` boundaries under `web/src`. Most are justified by hooks, browser storage, dialogs, dynamic navigation or mutations. The problem is boundary width and import graph, not the directive count.

| Component/route | Current | Proposed | Reason | Expected impact | Risk |
|---|---|---|---|---|---|
| App pages | Thin Server Components passing params to client controllers | Keep; optionally perform session capability read server-side through a shared server helper, then pass serializable capability model | Correct Next 16 route boundary; avoids duplicating route parsing | Better first paint/nav correctness | Must preserve CSP nonce, per-request dynamic render and BFF auth policy. |
| `OperatorConsole` routes | One large client island fetches all evidence after session hydration | Route-specific client controllers; render stable shell/header/server-compatible primitives outside interactive leafs | Reduce hydration/change coupling | Smaller route JS, fewer rerenders, clearer loading streams | Server reads must not bypass BFF/consistency or create waterfalls. |
| `components.tsx` | Entire primitive graph client-side | Split static vs interactive files | Hook-free display components need no client runtime | Bundle/request headroom for F09 | Import churn and accidental duplicate CSS; measure every batch. |
| `ConsoleShell` | Entire shell client-side | Server/static frame with compact nav and identity interactive islands | Drawer semantics need client; brand/frame do not | Lower hydration cost | Interleaving/slot complexity; preserve inert/focus behavior. |
| Overview | Client view with static financial documents and orientation/preferences | Keep view model client initially; later stream independent read-only sections only via the same authorized consistency contract | Independent domains already expose separate truth states | Potential LCP/JS improvement | A server-side parallel fetch can duplicate session/API reads and show inconsistent versions. |
| Developer/Recovery | Client controller + dynamic view | Keep dynamic view; migrate session boundary only | Optional heavy content already code-split | Preserves good route split | Extra loading UI must not imply empty evidence. |
| Event/Funding/Correction detail | Client fetch after hydration | Consider a server-rendered authorized initial snapshot plus client refresh/actions after shared server BFF helper exists | Detail IDs are stable and read-only snapshot helps first paint | Less loading flash | Read-your-writes, no-store, actor assertion and request references must be identical. |
| Root error/404 | Framework defaults | Add safe boundaries after two-request offset | Audit experiment proved each naïve boundary costs an initial script request | Better failure UX without budget regression | Do not raise 32-request gate; do not disclose errors/digests. |

Do not introduce broad Suspense fallbacks around financial groups that replace last verified evidence. Suspense is useful for initial independent route sections; refreshes must keep prior evidence with `EvidenceFreshness` and explicit historical state.

## 10. Data / State / API Architecture Review

### Fetch flow

The browser first obtains `/api/session`, then controller effects issue parallel same-origin reads. `readJSON` sets `no-store`, request ID and an eight-second timeout. BFF GET helpers enforce session and an allowlisted singleton query, then use a private workload credential and actor assertion. Responses remain no-store and propagate only bounded request/retry headers. This is appropriate; adding React Query is not justified by a missing feature and could obscure freshness/RYEW semantics.

Recommended improvement: standardize a small `useEvidenceResource` internal helper for generation, last-verified timestamp, retained evidence, explicit `UIDataState`, retry and single-flight pagination. It must be parameterized by a stable resource key and must not cache across tenant/session boundaries. Domain hooks still validate payload shape and interpret domain states.

### Mutation flow

The best pattern is `useTransferSubmission`: prepare canonical exact intent → persist tenant-scoped retry record → immediate ref lock → send CSRF/idempotency/request ID → classify final vs unknown → keep same body/key for unknown → refresh authoritative reads. Account and reconciliation flows use similar guarded patterns. Funding/correction controllers should converge on this pattern, including confirmation and precondition refresh.

No financial optimistic updates should be introduced. A newly returned authoritative funding record can be inserted in a list because the server returned it; a balance must still refresh from authoritative account evidence. An “accepted” or “running” command must not be rendered as “posted”, “matched”, or “delivered”.

### Error and retry flow

Current strengths: errors retain request references; empty, forbidden, offline and unavailable are distinct; same-filter failure can retain historical evidence; timeouts after dispatch become unknown. Remaining inconsistency is controller-specific implementation and missing route render boundaries. Normalize **mechanics**, not domain copy.

### Consistency and caching contract

- Browser and BFF reads stay `no-store`.
- Signed session consistency requirements remain bounded and server-controlled (`session.ts:11-20,49-65`; transfer BFF `:60-72`).
- Redis is never presented as balance authority.
- Prefetching financial details should be off by default until it can carry the same tenant/actor/consistency context and has a measured workflow benefit.
- Offline mode remains historical/read-only; no offline command queue.
- Saved views may store filters, never financial result snapshots.

### Duplicated logic to remove

Session fetch, online events and sign-out repeat across six consoles. Request generation is implemented differently across accounts, operations, funding and other controllers. Exact outcome/state helpers exist but are not uniformly used. Consolidate these narrow mechanics; retain domain-specific validation and wording.

## 11. UX / Operator Workflow Review

| Current friction | Proposed flow | Expected benefit |
|---|---|---|
| Navigation is a flat domain list and labels all events/developer/recovery as “Local tools” | Primary groups: **Work** (Overview, Accounts, Funding, Transfers, Approvals), **Investigate** (Reconciliation, Corrections, Events/Webhooks), **Platform** (Developer, Recovery); Local Status only local; Admin only authorized production. Use capability-derived visibility. | Faster role-specific orientation without weakening direct-route authorization. |
| Account → transfer → event → reconciliation context is split across lists and manual back links | Preserve a signed/bounded return context and add a shared “Related evidence” rail on detail views with status/freshness, not duplicated financial values. | Operators answer what happened without losing filters or opening DB/log tools. |
| Funding approval and posting are on the detail page but not a dedicated work queue | Add Approval Inbox with requester, age, evidence completeness, separation-of-duty reason and exact next action; detail retains immutable context. | Reduces missed approvals and makes role separation operationally visible. |
| Reconciliation mismatch links accounts but not a complete investigation path | Mismatch → affected account → bounded transactions/transfers at watermark → related events → correction request, with an investigation context token in URL. | Shorter, reproducible investigations while preserving immutable run scope. |
| Event detail explains delivery vs money but webhook lifecycle is elsewhere/not exposed | Events & Webhooks workspace: endpoint, delivery attempts, bounded error, related financial record; replay remains a separately authorized idempotent command. | Answers “was it delivered and what can I safely do?” without conflating posting. |
| Corrections/funding post are direct high-risk actions | Review current authoritative state → explicit immutable effect → confirm → pending/unknown/final result → related journal/balance refresh. | Reduces accidental activation and aligns all money-changing commands. |
| Filters vary by route and some are local-only | Shared list contract with URL filters, default sort label, count semantics, clear-all, cursor, export parity and back context. | Reproducible support cases and lower operator memory load. |
| Request references are copyable but there is no unified case bundle | “Create evidence bundle” exports a bounded manifest of related record IDs, timestamps, statuses, request references and redacted links—not mutable data or secrets. | Safer escalation to support/SRE/finance and clearer handoffs. |
| Overview states facts but recommended next action is not systematic | Add deterministic next-action rules from current evidence: investigate mismatch, review pending approval, retry unavailable evidence, or continue onboarding. | Operators know what to do without AI or hidden heuristics. |
| Failure → diagnostics/recovery requires knowing which tool applies | State panels link only to the relevant capability: retry this read, Local Status for local dependency failure, Recovery for verified backup/restore evidence, or support bundle. | Less random retrying and lower risk of inappropriate recovery actions. |

The current console can already answer much of “what happened?” for a known record. It is less effective at “find the record,” “work the queue,” “preserve an investigation,” and “handoff the evidence.” M08 should focus on those operational questions rather than adding more unrelated sidebar links.

## 12. Accessibility Gap Analysis

### Already verified

- Full authored-color WCAG A/AA automated rule matrix across populated routes.
- No automatically detected serious/critical issues in critical workflows.
- Keyboard-only transfer unknown-result flow and account lifecycle dialogs.
- Compact navigation modal behavior, focus trap, Escape, background `inert`, and trigger focus restoration.
- Account conflict outcome now deterministically receives focus after this audit.
- 320 CSS-pixel reflow, 200%-equivalent width, text spacing, phone rotation, 44 CSS-pixel targets.
- Forced-colors and reduced-motion behavior.
- Visual truncation retains complete accessible/copyable identifiers.
- Balance and history, financial and delivery, current and historical evidence are announced as separate states.
- Native download/export review and scope disclosure.

Evidence: `web/tests/e2e/accessibility.spec.ts`, `phase9-matrix.spec.ts`, `responsive.spec.ts`, account/reconciliation/recovery/domain specs, and the 136/136 browser result in this audit.

### Remaining high-value gaps

1. **Real screen-reader task interpretation:** verify NVDA/Firefox and NVDA/Chrome on Windows, plus VoiceOver/Safari where an authorized device exists. Automated name/role checks cannot prove dense evidence/timeline comprehension.
2. **Dense table navigation:** validate that screen-reader users can identify sticky first-column context after horizontal movement, understand result count/sort, and reach appended cursor rows without focus loss.
3. **Funding panel focus:** choose inline or modal semantics and implement deterministic entry/return focus (F14).
4. **High-risk post confirmations:** make funding/correction posting keyboard-modal and announce current authoritative evidence, not merely button busy state (F08).
5. **Live-region discipline:** `StatePanel` currently uses `role=status` for most non-error states (`components.tsx:37-39`). Static empty/permission content may be over-announced; define static vs dynamic variants.
6. **Pagination focus:** after “Load more,” provide a stable result summary and optionally move focus to the first newly appended record only when initiated by keyboard and validated with users.
7. **Timeline semantics:** manually verify event/transfer/reconciliation stage order, missing steps, and “financial status separate” language without visual connectors.
8. **Error summary coverage:** form fields have hints and invalid state, but every multi-field error path should focus a summary and link errors to exact fields as account lifecycle already does.
9. **Real browser zoom and OS contrast:** CSS-width emulation is strong evidence, not proof of browser zoom UI, Windows High Contrast combinations, OS scaling or browser chrome interactions.

### Manual/external verification required

- Named accessibility reviewer and device/browser/assistive-technology matrix.
- Physical phone/tablet landscape and touch exploration.
- Production IdP/step-up flows with real MFA and timeout behavior.
- Any public site (M10) and customer-facing surface must receive its own audit; console results do not transfer automatically.

## 13. Performance Review

### Current measured state

| Gate | Budget | Audit result |
|---|---:|---:|
| Constrained-4G compact LCP | ≤2,500 ms | 1,452 ms |
| Interaction INP | ≤200 ms | 24 ms |
| CLS | ≤0.1 | 0 |
| Initial document/script/style/fetch requests | ≤32 | 32 |
| Initial API requests | ≤8 | 5 |
| Max repeated API frequency through nav | ≤2 | 2 |
| Max long task | ≤250 ms | 85 ms |
| Observed long-task total | ≤1,500 ms | 85 ms |
| Largest JS chunk | ≤350,000 bytes | 229,156 bytes |
| Total JS | ≤2,000,000 bytes | 1,021,254 bytes |
| Largest/total fonts | ≤160,000/320,000 bytes | 0/0 |

The gate matters in architectural decisions. During this audit, adding root error and not-found files increased initial requests to 34 while other metrics stayed healthy. The files were reverted and the gate returned to 32. This is concrete evidence that route-fallback work must be paired with client-boundary or chunk reduction, not a reason to increase the budget.

### Recommendations

- Preserve current budgets as ceilings. Add per-route route-JS output to CI when the build exposes stable machine-readable manifests; track Overview, Funding, Corrections and Developer separately.
- First performance refactor should split static display primitives from `components.tsx` and the shell from compact interaction. Target at least two fewer initial scripts before F09.
- Consolidate session lifecycle to avoid accidental extra `/api/session` or account-directory reads on route transitions; maintain the current “no API path more than twice” budget.
- Keep dynamic import for Developer and Recovery. Evaluate Corrections detail and explainability timeline only after route bundle measurement; do not lazy-load primary evidence needed to answer the page's question.
- Keep 25-row cursor pages and bounded 100-account selectors. For larger lists, add server filters/sort before virtualization.
- Track CSS transfer/parse size as a new measurable budget after extraction. Proposed initial ceiling: current production CSS bytes as baseline, no increase per migration; reduce only with verified unused-rule evidence.
- Do not add a custom web font without a demonstrated design/accessibility need. The zero-font bundle is a performance and privacy strength.
- Do not add blanket link prefetch for financial detail routes; measure navigation latency and ensure actor/consistency context before any change.

## 14. Testing Gap Analysis

| Layer | Existing strength | Actual gap / next evidence |
|---|---|---|
| Unit | Exact money, financial UI, intent parsing, state contracts, recovery boundaries | Add generation/race/single-flight tests for transfer/run/list pagination and session provider extraction. |
| Component | Server-rendered markup contracts for states/labels/evidence | Add focused tests for new ConfirmationDialog, live-region modes, Money/Identifier/Timestamp and Funding open/close focus. Avoid snapshot-only tests. |
| Integration/BFF | Extensive signed session, CSRF, Host/origin, scope, payload, timeout, sanitizer and credential tests | Add any new list sort/filter/export query to strict allowlist/rejection tests; telemetry needs redaction/schema tests. |
| E2E | 136 mocked production-build cases cover core flows/states | Add role/environment navigation matrix, funding/correction guarded post, stale response ordering and unified investigation return context. |
| Accessibility | axe, keyboard, 320/reflow, text spacing, forced colors, reduced motion, touch targets | Named manual NVDA/VoiceOver/real zoom/physical device task review. Add screen-reader outcome scripts, not just generic audit. |
| Visual regression | Critical populated, empty, unavailable, offline, denied, unknown, mismatch, dialog and responsive baselines | Add route error/404 only when F09 lands; add approval inbox/webhook states with M08. Human review remains required when updating baselines. |
| Performance | Web vitals, requests, API frequency, long tasks, JS/font sizes, large bounded list | Add route-specific JS/CSS manifest reporting and a managed real-user plan under M13; keep current ceilings. |
| Real-stack | Isolated mutation-guarded account lifecycle across browser/BFF/API/Postgres/Redis with exact DB proof | Re-run against an explicit isolated project and exact clean commit after current source changes. Never point it at normal/shared Compose. |
| Physical/manual | Repository correctly disclaims it | Execute approved device/AT matrix, real IdP/MFA flows, production browser/CSP/telemetry, and visual review with named evidence. |

The test suite did expose real problems in this audit: account conflict focus was nondeterministic, two selectors had drifted, and naïve route boundaries violated the request budget. Fixes preserved or tightened assertions; no threshold, financial invariant or accessibility rule was weakened.

## 15. Existing Roadmap Reconciliation

| Recommendation | Existing master phase/task | Planned? | Current status | What is still missing |
|---|---|---|---|---|
| Canonical `web/`; coordinated legacy removal | M00 baseline + existing phase-11 cleanup audit D-01 | Existing task requiring execution | Decision established; removal not done | Atomic removal, contract/doc inversion, full qualification. |
| Role/environment-aware IA and missing workspaces | M08 | Existing task requiring refinement | Partial | Capability-derived nav; Approvals, Webhooks, Admin; grouping/context. |
| Standard list/detail contract | M08 lines 626-655 | Existing planned task | Partial by domain | Sort/count/search/date/URL/export/back context parity. |
| Controller/session architecture | M08/M09 enabling work | Newly discovered frontend work | Not started; funding generation fix complete | Session boundary, route controllers, async race guards. |
| Shared UI foundation | M09 | Existing planned task requiring refinement | Partial | Static/client split, guarded dialog, Money/Identifier/Timestamp, live-region policy. |
| CSS restructuring | M09 | Existing task requiring refinement | Not started structurally | Layer/file ownership and incremental migrations. |
| High-risk post confirmation | M05/M06 control UX + M09 accessibility | Newly discovered frontend work | Not started | Funding/correction precondition refresh and confirmation evidence. |
| Safe route failures within budget | M03 truthfulness + M09 | Newly discovered frontend work | Blocked by current request budget until offset | Two-request reduction, non-disclosing boundary tests. |
| Browser telemetry | M13, depends on M12 | Existing planned area requiring frontend detail | Not started for managed browser | Sanitized schema, collector, ownership, retention/consent. |
| Physical/manual accessibility | M09 exit gate | Existing external/manual gate | Not proven | Named reviewer/device/AT evidence. |
| Public website | M10 | Existing planned task | Pending | Separate `site/`, content/legal/consent/monitoring; do not reuse console IA blindly. |
| AI evidence assistant | Historical future Phase 8; subordinate to current master | Already planned, intentionally later | Deferred | Read-only tool gateway, authorization, citations, evals and zero mutations. |
| Production identity/tenancy | M11 | Existing planned/external work | Partial | Managed lifecycle and real Cognito MFA/PKCE/grants/revocation/isolation. |

Older roadmap language conflicts with current sequencing in places. Per the master register, those documents remain historical evidence; M07 active and M08/M09 partial control this report. No duplicate roadmap is created here—the execution items below refine those milestones and identify genuinely new frontend debt.

## 16. Future Scope

Classification: **A** already planned, **B** partially implemented, **C** new proposal, **D** intentionally deferred, **E** not recommended.

| Rank / idea | Class | WHY / WHAT | WHERE / HOW | WHEN | WHO | Backend/API dependency | Risk | SUCCESS |
|---|---|---|---|---|---|---|---|---|
| 1. Investigation workspace | C | Operators need a reproducible case spanning records. Create a bounded workspace containing links, filters, notes taxonomy and evidence manifest—not copied mutable truth. | New `/investigations/[id]`; extend related-evidence rails. Server stores authorized record references, owner, timestamps and immutable query context. | NEXT after M08 list contract | Product + Frontend; Backend/Finance support | Investigation entity, scoped relation lookup, audit events | Stale snapshots, tenant leaks, evidence overcollection | Median time-to-explain drops; cases reopen with same authorized context; zero copied balances as authority. |
| 2. Global ledger search | C | Finding the starting record is the biggest cross-domain friction. Search exact IDs/references first, then bounded text fields. | Header command/search route; backend fan-out/search index returns typed result stubs with source and freshness. | NEXT after query contracts | Frontend + Backend + Security | Tenant-scoped indexed search, rate/length controls | Sensitive enumeration, stale index, expensive queries | ≥90% known-ID searches reach correct detail in one action; authorization leakage tests pass. |
| 3. Approval inbox | A | Funding/correction approvals need an explicit queue with separation-of-duty reasons and age. | `/approvals`; reuse funding/correction detail actions; backend typed union/page and policy evidence. | NOW/NEXT M08 after F08 | Product + Frontend + Financial | Cursor list, role/step-up/actor evidence | Accidental approval, queue ambiguity | Pending items found; self-approval visibly blocked; decision lead time measured. |
| 4. Events & webhook management | A/B | Event evidence exists; endpoint lifecycle/replay platform exists but operator UI is incomplete. | Evolve `/events` into tabs/contexts for events, endpoints and attempts; replay is separate guarded command. | NEXT M07→M08 | Developer Platform + Frontend + Security | Safe endpoint metadata, delivery attempts, replay idempotency/authorization | Secret exposure, replay storms, financial conflation | Delivery failures resolved without DB access; replay duplicates zero. |
| 5. Saved operational views | C | Repeated investigations need stable filters without copying results. | Save named allowlisted query objects for accounts/transfers/events/approvals; URL remains canonical/shareable. | NEXT after F05 | Product + Frontend + Backend | Per-operator preference storage and validation | Stale expectations, filter leakage | Repeated workflows take fewer filter actions; saved view opens current evidence. |
| 6. Related-record graph | C | Linear links do not show transfer–journal–event–reconciliation–correction relationships at once. | Detail-side graph/list with deterministic typed edges; accessible ordered alternative is primary. | LATER after investigation API | Frontend + Backend + A11y | Bounded relation endpoint with edge provenance | Visual complexity, unbounded graph, screen-reader exclusion | Operators identify causal record faster; every edge links deterministic evidence; bounded node count. |
| 7. Audit/evidence bundles | C | Support/finance handoffs need safe, repeatable evidence. | Export a manifest/ZIP of authorized CSVs and metadata with hashes, schema version, redaction and expiry; UI reviews exact scope first. | NEXT after investigation model | Backend + Security + Frontend + Finance | Async bounded export job, signing/retention policy | Data exfiltration, oversized exports, stale evidence | Bundle generation audited; no secrets/free text leakage; recipients reproduce case. |
| 8. Role-aware workspaces | B | Scopes already gate data/actions, but navigation and summaries do not adapt. | Capability model drives nav, Overview queues and safe actions; direct routes still enforce server auth. | NOW M08 | Frontend + Product + Security | Session capability metadata already exists; admin policy later | UI treated as authorization, hidden discoverability | Role matrix passes; irrelevant links removed; server denials unchanged. |
| 9. Incident/notification centre | A | M13 needs operational ownership; operators need durable incidents, not transient browser toasts. | Read-only incident feed linking alerts, request refs and affected capabilities; acknowledgement is audited, not financial mutation. | LATER M12/M13 | SRE + Product + Frontend | Alert/incident service, SLOs, paging ownership | Alert fatigue, sensitive logs | Acknowledgement/MTTA improves; every incident has owner/runbook/evidence. |
| 10. Reconciliation analytics | C | Immutable runs exist; trends could surface recurring control failures without declaring truth. | `/reconciliation/analytics`; server-aggregated mismatch classes, duration, coverage, watermark gaps; drill into runs. | LATER after volume evidence | Financial + Backend + Frontend | Safe aggregate endpoint, retention semantics | Misleading green charts, cross-currency aggregation | Faster repeat-mismatch detection; dashboards never replace latest authoritative run. |
| 11. Deterministic next-action engine | B | Overview/onboarding has guidance, but operational next actions are not uniform. | Server/domain rules return typed recommended actions tied to evidence states and permissions; no ML. | NEXT M08 | Product + Financial + Frontend | Optional server rule endpoint or client pure rules over existing typed evidence | Wrong advice, hidden policy | Actions are explainable, permission-safe, and reduce dead-end screens. |
| 12. Read-only AI investigation assistant | A/D | AI can summarize/link immutable evidence after deterministic investigation APIs exist. It cannot be authority. | Separate internal copilot using schema-validated read-only tools; every sentence cites record IDs/stages and uncertainty. | LATER after M13 evidence maturity and AI safety foundation | Security + AI Platform + Product + Financial | Read-only gateway, authorization propagation, audit, evals | Hallucination, prompt injection, data leakage, automation bias | AI-initiated mutations exactly zero; citation correctness and abstention thresholds pass. |
| 13. Natural-language filter translation | D | Could reduce query friction, but only after global search/filter schema and AI controls. | Convert text to a previewed allowlisted filter AST; operator confirms before execution; no free-form backend query. | LATER after #2/#12 | Frontend + AI + Security | Filter grammar/tool and evaluation corpus | Incorrect/overbroad filters, leakage | High exact-filter accuracy; preview edits low; no unauthorized field/query. |
| 14. Multi-tenant administration | A | Production onboarding/revocation and permissions require managed UI. | `/admin` only after M11 APIs; tenant/operator lifecycle, grants, revocation, audit; current route stays not-found. | LATER M11/M12 | Security/Platform + Product + Frontend | Managed tenant/operator APIs, step-up, policy and audit | Highest authorization blast radius | Isolation/revocation tests, four-eyes policy where required, external security approval. |
| 15. Developer schema explorer | B | Developer metadata/OpenAPI download is strong; a safe browsable contract improves onboarding. A credentialed write runner is unnecessary now. | Enhance `/developer` with route/schema/error/retry navigation and copyable examples generated from OpenAPI. | NEXT M07 | Developer Platform + Frontend | Existing versioned metadata/OpenAPI | Contract drift, accidental secret entry | Partner completes integration from docs; generated artifact convergence stays green. |
| 16. Multi-currency UI readiness | D | Current pilot is INR and correctly blocks mixed aggregation. Premature UI would imply unsupported ledger policy. | First extract Money/currency-safe layouts and test long formats; enable currencies only with financial/backend policy. | LATER after explicit product decision | Financial + Product + Backend + Frontend | Currency metadata, rounding/policy, reconciliation and funding support | Severe financial misstatement | Each currency has exact policy/tests; no cross-currency totals without explicit conversion authority. |
| 17. Public marketing/developer site | A | Console should not carry public positioning, trust/legal or acquisition. | Separate `site/` per M10; share tokens later through proven package boundary, not console components wholesale. | NEXT/LATER after M07/M09 | Product/Marketing/Legal + Design/Frontend | Content, consent, abuse controls, hosting/monitoring | Claims outrun evidence, privacy/legal gaps | Evidence-backed content, accessibility/legal review, monitored pilot requests. |
| 18. Real-time financial status streaming | E now | Current explicit refresh/polling is bounded and truthful. Streaming adds ordering/reconnect complexity without evidence of operator harm. | Do not add for balances/posting now. Reconsider event/incident notifications only with versioned resumable streams and fallback refresh. | Not recommended until measured need | Backend/SRE/Frontend | Versioned event stream, auth, resumption, consistency | Out-of-order/stale truth, connection load | Only reconsider if measured latency impairs operations and invariant tests exist. |

Also intentionally deferred: localization and customer-facing embeddable ledger views. Both require explicit product, legal, authorization and support models. They should not block M07–M09.

### AI hard boundary

AI may explain, summarize, relate records, propose filters or suggest an investigation path. It must never post/reverse money, approve a funding/correction, mutate evidence, fabricate a missing step, claim reconciliation passed, bypass scopes, or turn a probabilistic answer into a status badge. The UI must keep deterministic record citations adjacent to each interpretation and provide a direct “open evidence” path. Free text and AI traces need a retention/privacy decision before use.

## 17. Top 10 Recommended Actions

1. **P1/M — Guard funding and correction posting** with refreshed authoritative evidence, explicit immutable effect, deterministic focus, idempotent retry and stale-conflict handling (F08).
2. **P1/M — Eliminate async evidence races** in transfer/run/detail and cursor append loaders with keyed generations and immediate single-flight locks (F10).
3. **P1/M — Introduce the narrow shared console session boundary** and prove one session lifecycle without financial caching or performance regression (F03).
4. **P1/L — Split `OperatorConsole` by route/domain controller** after actions 2–3, preserving domain hooks and independent truth states (F02).
5. **P1/M — Make IA capability-, role- and environment-aware** and implement the M08 approval/webhook/admin ownership model without adding a flat link pile (F04).
6. **P1/L — Define and implement the M08 list/detail contract** beginning with Approvals and using Events/Accounts as reference; add server query support before UI affordances (F05).
7. **P1/L — Establish `src/ui` static/interactive primitive boundaries** with ConfirmationDialog, Money, Identifier and Timestamp; earn at least two initial-request slots (F07).
8. **P1/L — Incrementally split CSS ownership** by foundations/primitives/features, starting Funding and Corrections, with unchanged visual/performance gates (F06).
9. **P1/M — Add safe route/404 boundaries only after action 7 creates request headroom**, retaining ≤32 initial requests and no sensitive error disclosure (F09).
10. **P1/M — Execute the existing coordinated legacy removal** as a separate clean change with full root/web/Compose/security qualification (F01).

M07 exact-commit developer work continues in parallel where it does not touch shared console primitives. Browser telemetry design can begin with SRE/Security, but implementation waits for M12/M13 collector, policy and ownership.

## 18. Execution Roadmap

### NOW — current M07–M09 foundations

Parallel lane A — financial command safety:

1. F08 guarded funding/correction post.
2. F10 loader generation/single-flight.
3. Add targeted unit/E2E/keyboard/unknown-outcome tests.

Parallel lane B — architecture foundation:

1. F03 shared session boundary.
2. Route-by-route F02 controller extraction: Overview → Transfers → Reconciliation → Accounts.
3. Prove API/request/performance budgets after every extraction.

Parallel lane C — UI foundation:

1. F07 static vs interactive primitive graph; ConfirmationDialog and evidence primitives.
2. F06 CSS extraction beginning with shared foundation, then Funding/Corrections.
3. Recover at least two initial requests, then F09 route fallbacks.

Parallel lane D — product/IA definition:

1. Product, operations and security agree role/environment nav matrix.
2. Backend/frontend define Approval Inbox and webhook list/detail query contracts.
3. M08 list contract and cross-domain investigation context are written as acceptance criteria.

The coordinated legacy deletion should run as a separate clean branch/change because its root dependency/Compose/contract blast radius is unrelated to active UI refactors.

### NEXT — enabled by foundations

- Approval Inbox, Events & Webhooks, capability-driven navigation and standardized lists/details.
- Global exact-ID search, saved views and investigation workspace in that order.
- Evidence bundles after investigation authorization/retention model.
- Developer schema explorer and remaining M07 recipes/endpoint verification.
- Complete CSS feature migration and manual accessibility remediation.
- Safe browser telemetry once M12/M13 endpoint, policy and owner exist.

### LATER — strategic, non-blocking

- Relationship graph, reconciliation analytics, incident centre.
- Managed administration with M11/M12.
- Separate public site M10.
- Read-only AI assistant and natural-language filters only after deterministic evidence/search and AI safety gates.
- Multi-currency/localization/customer embeds only after explicit product/financial/legal decisions.

### EXTERNAL GATES

- Real Cognito/MFA/PKCE/grant/revocation/isolation proof.
- Managed AWS account, Terraform review, DNS, credentials, budgets and production CSP/telemetry.
- Named manual accessibility reviewer, physical devices, NVDA/VoiceOver and real zoom.
- Finance/compliance/legal decisions and external security/penetration review.
- Provider-backed RDS PITR/DR, managed load/fault tests and exercised incident runbooks.
- Design-partner contracts/provisioning/operation and production go/no-go authority.

## 19. Who Does What

| Role | Accountable work | Supporting responsibilities | Cannot self-certify |
|---|---|---|---|
| Frontend Engineer | Loader races, shared UI primitives, route controllers, CSS migration, guarded dialogs, list UX | Unit/component/E2E implementation evidence, bundle measurement, docs | Finance policy, security approval, production readiness |
| Senior/Staff Frontend Engineer | Target architecture, session/client boundaries, IA technical model, performance/bundle tradeoffs, legacy change coordination | Review invariants, sequence refactors, accept/defer P1 debt explicitly | External accessibility, legal, provider or release approval |
| Product Designer | Operator IA, hierarchy, queue/investigation flows, confirmation copy, responsive/task design | Visual baseline review, semantic status vocabulary, manual usability | Financial correctness or authorization policy |
| Accessibility specialist | Screen-reader/keyboard/zoom/contrast task plan and manual findings | Dialog/table/timeline semantics and remediation acceptance | Production security or finance gates |
| Backend/API Engineer | Typed list/search/approval/relation/export APIs; preconditions; pagination/sort; actor/consistency propagation | BFF schemas, sanitizers, performance data sets | UI task usability or finance policy alone |
| Financial Engineer | Posting/correction/funding/reconciliation invariants and high-risk action acceptance | Exact effect language, precondition evidence, multi-currency decisions | Accessibility or production authorization alone |
| Security Engineer | Capability model, admin/step-up policy, telemetry allowlist/redaction, search/enumeration threat review | CSP/session/BFF tests, external review closure | Legal approval, finance sign-off or partner acceptance |
| QA/SDET | Test strategy, race/concurrency scenarios, real-stack exact-commit qualification, visual baseline governance | Device/browser matrix coordination and evidence archive | Manual accessibility specialist judgment or provider state |
| SRE/Platform Engineer | Managed collector, SLOs, alerting, incident centre, AWS/deployment/DR evidence | Browser telemetry transport, RUM budgets, runbooks | Product/finance/legal approval |
| Product Manager | M07–M09 scope, role workflows, success metrics, accepted deferrals, design-partner outcomes | Register synchronization and owner assignment | Technical/security/financial proof |
| Finance/Compliance | Approval separation, evidence/export/retention rules, reconciliation/control acceptance | Funding/correction copy and audit-bundle requirements | Engineering test results or legal conclusions |
| Legal | Privacy, retention, terms, consent, public claims, partner boundary | AI trace and telemetry policy | Technical security/financial correctness |
| External partner/design partner | Integration usability and operator workflow validation in approved environment | Feedback and graduation evidence | Production approval or internal control sign-off |

## 20. Completion Matrix

| Current-scope task | Status | Evidence now | Evidence required for COMPLETE |
|---|---|---|---|
| Canonical frontend decision | COMPLETE | `web/` is supported/current; dashboard is excluded legacy | Maintain this decision in architecture docs/register. |
| Coordinated legacy frontend/slice removal (F01) | NOT STARTED | Existing D-01 audit only | Atomic deletion, docs/contract inversion, root/web/Compose/security qualification. |
| Funding authorized-account truth states (F12) | COMPLETE | Generation, loading/error/scope completeness and fail-closed selector; two new unit tests; full gates pass | Requalify if picker/query contract changes. |
| Account conflict focus race (F13) | COMPLETE | Explicit focus ownership; full E2E conflict case passes | Requalify if dialog/outcome flow changes. |
| README/master phase reconciliation (F16) | COMPLETE | README M07; M08/M09 evidence updated and remain partial | Keep synchronized on phase commits. |
| Transfer/reconciliation/pagination race guards (F10) | NOT STARTED | Risk identified in exact loaders | Deterministic out-of-order and rapid-activation tests plus implementation. |
| Guarded funding/correction post (F08) | NOT STARTED | Direct post controls identified | Evidence-refreshing confirmation, focus/a11y, conflict, duplicate and unknown tests. |
| Shared console session boundary (F03) | NOT STARTED | Six repeated implementations mapped | One reviewed boundary, full scope/session/offline tests, request/performance budgets. |
| Route-specific Operator controllers (F02) | NOT STARTED | Target map defined | Domain ownership split, no cross-domain races, full qualification and architecture doc. |
| Role/environment-aware navigation (F04) | IN PROGRESS | Scope gating exists; static nav remains | Agreed capability matrix, route/nav tests, Local/Admin rules, direct-route fail closed. |
| Standard M08 list/detail contract (F05) | IN PROGRESS | Accounts/Events/exports/deep links partially meet it | Every M08 route meets documented applicable criteria or explicit not-applicable rationale. |
| UI primitive foundation (F07) | IN PROGRESS | Eleven shared primitives and tokens exist | Static/client split, recurring guarded/evidence primitives, migration and tests. |
| CSS architecture migration (F06) | NOT STARTED | Target/migration plan and visual gates exist | Owned layered files, reduced duplication, no specificity/budget/visual regressions. |
| Safe route error/404 UI (F09) | BLOCKED | Prototype was correct in disclosure terms but raised initial requests 32→34 and was reverted | Offset two initial requests, then pass security, render-failure, browser and performance gates. |
| Funding panel focus semantics (F14) | NOT STARTED | Gap identified | Product semantic decision plus open/close/Escape/focus/axe tests. |
| Browser telemetry (F11) | BLOCKED | Request references exist, managed telemetry does not | M12/M13 collector, policy, owner, allowlist/redaction tests, reviewed retention/consent. |
| Large-volume list policy (F15) | IN PROGRESS | Bounded cursor APIs and performance list test | Server sort/filter/query parity, scale dataset evidence, accessible interaction budget. |
| Automated accessibility/responsive/visual qualification | COMPLETE for current mocked local build | 136/136 browser suite passes | Requalify exact commit after changes; does not close manual gate. |
| Manual/physical accessibility | BLOCKED | Correct repository disclaimer | Named reviewer, approved device/AT matrix, findings closed/accepted. |
| Asset and browser performance gates | COMPLETE for current working tree | Asset budget and 2/2 browser performance pass | Exact clean-commit evidence for release candidate. |
| Real-stack frontend qualification | DEFERRED for this audit | Harness and historical evidence exist; prerequisites intentionally absent | Explicit isolated project/mutation approval/seed/run ID, clean exact commit, passing report. |
| M08 Operator-console IA | IN PROGRESS | Master register `PARTIAL` | All M08 exit criteria and exact-commit evidence; finance/ops/support/engineering task validation. |
| M09 Unified design/accessibility | IN PROGRESS | Master register `PARTIAL` | Shared foundation, manual accessibility and exact-commit phase exit evidence. |

`BLOCKED` here describes a concrete dependency for the individual task, not the overall frontend: safe fallbacks require bundle headroom; telemetry requires managed M12/M13 policy/infrastructure; manual accessibility requires authorized humans/devices.

## 21. Definition of Done

The LedgerSync frontend may be called **frontend engineering complete** only when all of the following are true for one clean exact commit:

### Product and architecture

- `web/` is the documented canonical console and the coordinated legacy slice is removed or explicitly accepted by a named owner with expiry/rationale.
- Route → controller → domain hook/view model → presentational section → UI primitive → BFF architecture is documented and reflected in major routes.
- No accepted P0 frontend issue remains.
- Each accepted P1 issue is resolved or explicitly deferred in the master register with owner, reason, risk and follow-up milestone.
- `OperatorConsole` no longer owns unrelated domain request graphs; shared session lifecycle does not cache financial evidence.
- Async reads/pagination cannot let old responses overwrite/duplicate current evidence.

### Financial and security UX

- Integer minor-unit/exact string money and explicit currency are retained end-to-end.
- Financial, delivery, reconciliation and recovery evidence remain separate and authoritative.
- No financial mutation is optimistic; unknown outcomes retain exact body/key and safe retry.
- Funding, transfer, lifecycle, reconciliation and correction high-risk actions have current-evidence confirmation proportional to risk.
- Scope/tenant/actor/CSRF/Host/idempotency and BFF/private credential boundaries remain independently enforced and tested.
- No sensitive identifiers, amounts, free text, credentials, payloads or error details enter logs/telemetry outside a reviewed allowlist.

### UI foundation and operator workflows

- Recurring primitives have one maintained implementation where semantics match; static primitives do not force avoidable client graphs.
- CSS has explicit foundation/layout/primitive/pattern/feature ownership and a stable responsive/cascade strategy.
- Navigation is role/environment aware; direct-route authorization remains server-enforced.
- Applicable list screens meet M08 sort/search/filter/date/clear/URL/cursor/count/state/export/deep-link/back-context requirements.
- Detail screens expose applicable exact amount, IDs, status/history, actor, policy, posting/balance/event/reconciliation evidence, UTC, correlation, safe actions and related records.
- Operators can answer what happened, who/what changed, whether evidence is current, what action is safe and how to hand off evidence without database access.

### Accessibility and responsive

- Automated axe/authored-color, keyboard, focus, 320/400%-reflow, text-spacing, touch target, forced-color, reduced-motion and visual matrix pass.
- Named manual screen-reader, real zoom, physical-device and high-contrast tasks are complete for critical flows, with findings resolved or accepted by an authorized owner.
- Dialog entry/exit/outcome focus and live announcements are deterministic; long identifiers and dense tables/timelines remain understandable.

### Qualification and evidence

- ESLint and Next TypeScript/production build pass.
- All unit, security, integration, E2E, visual and browser performance tests pass without weakened assertions/budgets.
- Asset budgets remain ≤350 KB largest JS, ≤2 MB total JS, ≤160/320 KB font, or are deliberately tightened with evidence.
- Compact performance remains ≤2.5 s LCP, ≤200 ms INP, ≤0.1 CLS, ≤32 initial requests, ≤8 initial API requests, ≤250 ms max long task.
- Real-stack browser flow passes in the approved isolated environment when available; production claim requires managed-environment evidence.
- Visual baseline changes have human review; generated reports/noise are not accidentally committed.
- README, design/QA evidence, architecture docs and master register agree.
- Working tree contains no untracked frontend release blocker and the exact-commit CI/release evidence is preserved.

This contract can establish **frontend engineering completeness**. It cannot by itself establish LedgerSync product production readiness; M11–M17 and their human/provider/legal/partner/release gates still control that claim.

## 22. Final Completion Verdict

PARTIALLY COMPLETE

Completed now: repository truth and canonical frontend decision; comprehensive architecture/design/UX/a11y/performance/testing/roadmap audit; funding account-scope truthfulness and fail-closed entry; deterministic account-conflict focus; stale browser selector corrections without weakened assertions; README/master phase reconciliation; clean lint, 100 unit/security tests, production build, 136 browser/a11y/visual tests, two browser performance tests and asset budgets.

Remaining frontend code/product work: P1 guarded funding/correction posting, async race guards, shared session and route-controller boundaries, role/environment-aware IA, M08 list/detail consistency, UI/CSS foundation extraction, safe route fallbacks within the existing request budget, and selected operator investigation workflows. Frontend, Staff Frontend, Product Design, Accessibility, Backend/API, Financial Engineering, Security and QA share those responsibilities as mapped above.

Remaining external work: real-stack requalification for the exact clean commit, manual screen-reader/physical-device review, managed identity/tenancy/infrastructure/telemetry/DR evidence, finance/compliance/legal/security approvals, design-partner operation and production release authority. Those are not frontend code claims and were not fabricated in this audit.
