# Local-product Phase 9 — operator experience convergence

**Result:** `PASSED`

**Verified:** 2026-08-25T00:04:42Z

**Gate:** [LPC-090](../pilot/local-product-completion-gates.md)

**Candidate:** Phase 9 working tree based on `32bf5bd`; the resulting Phase 9 commit binds this evidence to the implementation.

**Boundary:** every route and supported state in the loopback-only local operator product, the real PostgreSQL financial service path, the authorized warm-cache balance reader, local Windows Chromium, and the exact pinned Playwright Noble Linux image. This is not external-device, public-network, or production capacity evidence.

## Converged product behavior

- The canonical `DESIGN.md` type scale is now enforced at 32/24/18/15/16/14/12 pixels for page, section, body, label, metadata, and evidence roles. Compact evidence remains readable without restoring the prior 8–10-pixel body/table grammar.
- Shared explicit components now own evidence freshness, section headings, and numbered evidence markers. Variants describe actual states rather than contradictory boolean combinations.
- Verified account, balance, history, event, diagnostics, orientation, and explainability evidence remains visible while its own region refreshes. Each retained value has freshness or historical context; a failed refresh never converts an authoritative value to zero, empty, or current.
- The session shell renders after verified authorization, while independent account identity, balance, history, transfer detail, reconciliation, event, and local-status regions resolve separately. Generation/scope guards discard late responses from a prior account or route.
- Overview's five-record panel is explicitly a recent subset with a **View all transfers** route; it no longer claims the sliced records are the complete result.
- Transfer `q` and `status` filters execute at the authorized PostgreSQL query before keyset pagination. Cursor fingerprints bind the complete filter intent; an altered filter plus prior cursor is rejected instead of silently mixing result sets. Export review and displayed filters remain aligned.

## Navigation, responsive, and accessibility controls

- At compact widths the closed rail is inert and cannot receive focus. The open drawer is modal, traps Tab/Shift+Tab, closes with Escape/backdrop/close, restores focus to Menu, and prevents background scrolling.
- Required route matrix passed at 320, 390×844, 768×1024, 1024×768, 1366×768, 1440×900, and 1920×1080. Every populated route has exactly one main landmark, one visible H1, and zero page-level overflow.
- Data regions retain native table semantics and labelled horizontal inspection where exact columns cannot collapse. Content-width rules corrected account filters, Event detail, Local Status, Developer, and Recovery at rail-constrained tablet widths.
- All 13 populated routes passed a complete authored-color axe WCAG A/AA sweep. Keyboard, focus restoration, text spacing, 200% zoom, 320-pixel/400%-equivalent reflow, reduced motion, forced colors, and visible 44-pixel controls passed.
- UI boundaries use the reviewed 3.08:1 structural token. Status remains icon + text + color, and explicit rail backgrounds prevent translucent-parent contrast ambiguity.

## Performance and truth boundaries

- A fixed-cardinality OpenTelemetry histogram, `ledgersync.http.route.duration`, records only allowlisted route families with 200/500-millisecond boundaries. Raw URLs, tenant IDs, actor IDs, account/transfer IDs, amounts, and filters cannot become labels.
- Disposable real-PostgreSQL p95 was 20.1878 ms for transfers against the 500 ms target and 645 microseconds for authorized warm balance reads against the 200 ms target.
- A warm balance hit performs exactly one authorization query and zero PostgreSQL projection fetches. Cache-miss reauthorization remains deliberate protection against revocation TOCTOU; destination evidence retains its required post-read authorization.
- On a 390×844 viewport with 4× CPU throttling and constrained 4G, the overview-to-accounts journey passed with 29 initial requests/5 API calls, 32 total/7 API calls, LCP 1.192 s, INP 32 ms, CLS 0, and long tasks 168 ms total/114 ms maximum.
- Eight unnecessary requests were removed after the profiler showed Local tools routes being prefetched twice during provisional-to-verified shell transition. Automatic prefetch is disabled only for Local tools; primary financial routes remain prefetched.
- Production assets total 885,376 JavaScript bytes; the largest chunk is 229,156 bytes, below 2,000,000 and 350,000-byte limits. No fonts ship. Developer and Recovery route views remain lazy without changing evidence order.

## Automated evidence

| Gate | Result |
|---|---|
| Full Go suite | `go test ./... -count=1` passed |
| Full disposable PostgreSQL integration | Passed transfer filters/cursor intent, route privileges, financial behavior, and p95 measurements |
| Go static checks | `go vet ./...` passed |
| OpenAPI | Redocly canonical contract validation passed |
| Web unit/security/component | 78/78 passed |
| Full mocked browser suite | 118/118 passed |
| Expanded route/accessibility matrix | 8/8 passed across 13 routes and all required widths |
| Performance browser suite | 2/2 passed |
| ESLint, TypeScript, production build | Passed with zero lint warnings on Next.js 16.3.2 |
| Asset budgets | Passed: 885,376 total JS bytes; 229,156 largest chunk; 0 font bytes |
| Patch integrity | `git diff --check` passed with Windows line-ending notices only |

## Cross-platform visual evidence

- Windows: 35 approved baseline images were inspected through five approved contact sheets. The clean no-update comparison passed all 30 runnable cases.
- Linux: the exact pinned image `mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e` produced 27 inspected approved images. The clean comparison passed all 26 Linux-runnable cases; four guarded-action cases are explicitly Windows-only.
- Linux screenshot runs used a uniquely named Docker `--internal` bridge. The bridge provided truthful `navigator.onLine` semantics without an external route, its `Internal=true` property was verified, and it was removed in `finally`; zero such networks remain.
- Reviewed systematic changes are limited to canonical type tokens, 44-pixel targets, stronger boundary contrast, truthful freshness/progressive evidence, compact modal navigation, and content-width responsive wrapping. No unexplained per-image change was approved.
- Contact sheets are stored under `docs/design/qa/responsive/contact-sheets/phase9/`; the invalid diagnostic run using `--network none` is excluded because it correctly forced the product's offline state.

## Live supported-stack proof

- Exact candidate images rebuilt successfully and the browser/BFF smoke passed at `http://127.0.0.1:3000`.
- PostgreSQL, Redis, API, worker, and web are healthy; migrations and demo seed completed successfully; schema remains `000016_guided_read_models.up.sql`; outbox is 0 pending/0 dead; latest reconciliation is matched with 0 mismatches.
- A server-issued demo session returned one live posted transfer through the strict combined `q` + `status` BFF filter, and a non-identifier query was rejected with HTTP 400.

## Failures found and remediated

The expanded phase gate found and closed the following defects before approval:

1. Hidden compact navigation was keyboard-focusable and lacked modal focus containment.
2. Overview called a sliced result the end of available records.
3. Shared detail-loading flags raced parallel requests and allowed premature empty/unavailable conclusions.
4. Account refresh replaced retained verified evidence with a skeleton and omitted its timestamp.
5. Transfer filters covered only the loaded page and were not bound to pagination intent.
6. The implemented 8–10-pixel grammar diverged from the canonical design scale.
7. Account filters, Event detail, Local Status, Developer, and Recovery overflowed at specific required content widths.
8. Translucent rail descendants caused real axe contrast failures after typography convergence.
9. Duplicate Local-tools prefetch exceeded the hard 32-request initial budget.
10. Linux CI lacked baselines for Phase 5–8 routes and states.

Every item was fixed and cleanly retested; none is represented as an exception.

## Deliberate decisions and limitations

- The installed Next.js 16.3.2 and ESLint combination passes lint, type, build, security, E2E, and performance gates. No dependency or lockfile upgrade was justified, and no lint rule was weakened.
- Full-stack sustained HTTP/BFF/Redis capacity belongs to Phase 10. Phase 9 qualifies the real database financial path, warm balance reader, browser vitals, request count, and bounded runtime metrics needed to measure that acceptance honestly.
- External screen-reader/device-farm sign-off remains outside the local-only product boundary; keyboard, semantics, automated WCAG, zoom, reflow, forced-color, and reduced-motion evidence are current.
