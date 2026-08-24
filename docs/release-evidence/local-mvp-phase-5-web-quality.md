# Local MVP Phase 5 — responsive, accessible, and efficient web evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows workstation with production-built Chromium automation

**Evidence binding:** the Git commit containing this document

## What was proved

The local operator workspace uses one responsive component tree across compact,
tablet, desktop, and wide-desktop viewports. Exact financial evidence remains
available at high zoom, keyboard operation remains predictable, authored color
and forced-color states remain operable, and the production web bundle and
critical interaction path stay inside measured budgets.

## Responsive behavior

| Qualification | Observed result |
|---|---|
| Viewport matrix | Passed at 390×844, 768×1024, 1024×768, 1366×768, 1440×900, and 1920×1080 |
| 400% reflow equivalent | Passed at 320 CSS pixels with zero page-level horizontal overflow |
| 200% desktop zoom equivalent | Passed at 640 CSS pixels with exact account values and the primary filter action available |
| Portrait-to-landscape rotation | Exact signed-64-bit amount draft retained across 390×844, 844×390, and back |
| WCAG text-spacing override | Exact balance, evidence, controls, and return navigation remained available at 320 CSS pixels |
| Long labels and integers | Long account name/reference plus `9223372036854775807` minor units produced no page overflow |

Account, transfer, and reconciliation comparisons no longer render duplicate
desktop-table and mobile-card records. Each dataset has one semantic table in a
labelled, focusable horizontal-scroll region. Compact layouts expose a visible
scroll instruction and pin the identity column while the remaining exact fields
scroll inside the region rather than widening the page.

## Accessibility evidence

- All populated MVP routes passed automated axe checks for WCAG 2 A/AA,
  WCAG 2.1 A/AA, and WCAG 2.2 AA tags.
- Compact navigation opens by keyboard, closes with Escape, and restores focus
  to the invoking Menu button.
- Financial tables, copy controls, transfer confirmation, safe same-key retry,
  and primary actions are keyboard reachable without a focus trap.
- Copy completion is announced through a polite live region. Unknown transfer
  results are announced as status and retain the exact idempotency key.
- Visually shortened transfer routes expose complete source and destination
  account UUIDs in their accessible names.
- Compact primary actions and navigation meet the 44×44 CSS-pixel target.
- Reduced-motion and forced-color modes preserve operable evidence and status
  text. Status is never communicated by color alone.

Critical authored color ratios were calculated from the committed tokens:

| Pair | Contrast |
|---|---:|
| Primary ink / canvas | 15.62:1 |
| Muted text / paper | 6.17:1 |
| Rail text / rail | 16.05:1 |
| Rail muted text / rail | 10.91:1 |
| Success text / paper | 5.85:1 |
| Action blue / paper | 6.11:1 |
| Warning text / warning surface | 6.11:1 |
| Danger text / danger surface | 6.44:1 |
| Focus ring / paper | 4.00:1 |
| Focus ring / rail | 4.34:1 |

The earlier gold focus token measured only 2.18:1 against paper and was replaced
with `#B27100`; this correction is also recorded in `DESIGN.md`.

## Progressive rendering and measured efficiency

The browser budget ran separately from the parallel functional suite so CPU
contention cannot turn host load into false performance evidence. Under a
390×844 viewport, constrained 4G profile, and 4× CPU throttling, the measured
journey produced:

| Metric | Observed | Enforced budget |
|---|---:|---:|
| Largest Contentful Paint | 396 ms | ≤ 2,500 ms |
| Observed interaction latency | 32 ms | ≤ 200 ms |
| Initial Cumulative Layout Shift | 0.0777 | ≤ 0.1 |
| Longest initial main-thread task | 119 ms | ≤ 250 ms |
| Total observed long-task time | 119 ms | ≤ 1,500 ms |
| Initial bounded document/script/style/API requests | 24 | ≤ 32 |
| Initial API requests | 7 | ≤ 8 |
| Overview-to-accounts API requests | 11 | ≤ 12 |
| Maximum calls to one API path | 2 | ≤ 2 |

The production build contains 10 JavaScript chunks totaling 663,477 bytes; the
largest is 229,156 bytes against a 350,000-byte cap. The full JavaScript cap is
2,000,000 bytes. No webfont payload is shipped. A delayed bounded history of
100 immutable records uses progressive browser rendering and remains navigable.

## Reproducible checks

| Check | Result |
|---|---|
| ESLint | Passed |
| Web unit suite | 23 passed |
| Next.js production build and TypeScript validation | Passed |
| Functional, responsive, accessibility, state, and visual Chromium suite | 48 passed |
| Isolated throttled performance browser suite | 2 passed |
| Reviewed Windows visual baselines | 19 passed with five intentional compact updates |
| JavaScript and font artifact budgets | Passed |

## Boundary

This is automated production-browser evidence on the local workstation. It is
not a physical-device, Safari, mobile screen-reader, external device-farm, or
human assistive-technology certification. Those claims remain outside the
local-only MVP boundary; this phase does not weaken the production-pilot gates.
