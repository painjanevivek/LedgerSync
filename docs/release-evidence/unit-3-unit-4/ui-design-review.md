# Step 9 UI design review

Review date: 12 October 2026.

Decision: Ship this ordinary extension of the existing Chain-of-Custody Register. The fresh review marked the scheduler freshness correction resolved. That verdict applies to the named correction. It is not a full product release approval.

## Direction match

The work follows [PRODUCT.md](../../../PRODUCT.md), [DESIGN.md](../../../DESIGN.md), and the existing `.impeccable/design.json` sidecar. The direction remains a deep teal rail, mineral canvas, and bordered record surfaces. The extension uses existing tokens and shared components. The documentation reference was `C:/Users/ASUS/.codex/skills/impeccable/reference/document.md`.

- Developer has a collapsed **Distributed systems** section. It uses the shared disclosure, status, timestamp, copy, and state components.
- Event detail has a collapsed **Event order** section. It shows node identity, logical timestamps, publication attempts, and receive clocks in shared evidence lists and tables.
- Internal reconciliation detail has a collapsed **Snapshot boundary** section. It keeps database snapshot evidence separate from service observations.
- Technical explanations stay inside expanded details. No primary navigation item or failure-drill button was added.
- Historical, unavailable, and pending evidence has explicit wording. Publication history does not claim that a worker is running now. The snapshot wording excludes Redis and process state.

The scheduler badge uses a neutral tone and **Active at sample** wording. It changes to **Historical lease — refresh needed** when the sample expires. This prevents an old observation from appearing to prove current ownership.

## Evidence checked

| Evidence | Review result |
|---|---|
| [Developer before](ui-developer-before.png) | Used as the incumbent visual reference. The shell, page heading, contract actions, and existing reference content remain recognizable. |
| [1440px viewport](ui-developer-1440-viewport.png) and [full page](ui-developer-1440.png) | The new record fits the existing register. The focused summary has a visible outline. The viewport capture shows historical lease wording. The full-page capture shows the earlier sampled state. |
| [390px viewport](ui-developer-390-viewport.png) and [full page](ui-developer-390.png) | The shared mobile shell remains present. Observation fields stack in document order. Labels and the refresh control fit within the page width. |
| [Browser result after the correction](ui-review-fix-verified.txt) | One real-stack check passed. Its source checks initial collapse, Enter-key expansion, API observations, historical lease wording, evidence download, event order, and snapshot details. |
| Browser check source: `web/tests/system/distributed-evidence.real-stack.spec.ts` | At 1440px and 390px, it checks root overflow and runs Axe on the Developer page. The saved passing result records zero detected violations. |

Source review covered `DistributedSystems.tsx`, `DeveloperViews.tsx`, `EventOrderEvidence.tsx`, `EventViews.tsx`, and `ReconciliationViews.tsx`. Shared disclosure, evidence-list, timestamp, and responsive rules were read for context. This documentation review inspected saved evidence and source. It did not run new browser checks.

## Limits and existing drift

Impeccable context and detector launchers were unavailable. The engine was absent, and its cache was not writable. Project context was read directly. No detector score or detector pass is claimed.

The saved image review covers Developer. Event and reconciliation details have source and browser evidence, but no saved image comparison in this review. The Axe result does not prove complete WCAG conformance or manual screen-reader operation. These captures use the isolated study environment. They do not prove production availability.

Two existing shared rules differ from the written design direction. Disclosure surfaces use the small `--shadow-surface` token, although DESIGN.md specifies flat ordinary records. The shared Timestamp component inherits surrounding typography; the observed timestamps use sans text, although DESIGN.md assigns timestamps to the evidence mono role. These shared rules predate this extension. No repair or new design rule was made. The desktop evidence grid also leaves a tinted empty cell for the third observation; this follows the existing two-column pattern and is a minor layout limit.

## Documentation decision

Keep `DESIGN.md` and `.impeccable/design.json` as the incumbent system. This work adds evidence within existing pages and does not establish a new visual identity. Record this review as the scoped documentation artifact for [Step 9](../../plans/ledgersync-unit-3-unit-4-implementation-plan.md). Broader design drift requires separate review before a system refresh.

## Step 11: Visual fixture follow-up

The full regression run compared the current console with older navy and serif
baselines. The account view, shell, register style, and token files match their
Step 1 source fingerprints. See [the source comparison](visual-baseline-source-comparison.json).

The implementation review inspected representative account, event-detail, and
Developer captures. The current hierarchy and delivery warnings remain present.
The visual tests now open collapsed controls and use the current page labels.
They retain read-only restrictions and the mixed-currency refusal. A fixed test
clock makes the refreshed baselines repeatable.

All 72 affected browser checks passed with snapshot updates disabled. All 20 page
accessibility and reflow checks passed. See [the browser verification](final-web-browser-verified.txt)
and [page checks](final-web-accessibility-verified.txt). These results cover Windows
Chromium. They do not establish manual screen-reader conformance or pixel equality
on other platforms. The full combined browser run is recorded in the final validation.
