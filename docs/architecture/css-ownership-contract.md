# CSS ownership contract

## Context

LedgerSync's operator console started with one large global stylesheet. That file preserved a coherent visual language, but it made an apparently local change risky: a selector for one workflow could be separated by thousands of lines from its responsive override, and reviewers could not tell which team or feature owned it.

The stylesheet is now organized by responsibility while preserving the original cascade. This is a structural change, not a redesign. The browser must render the same visual evidence before and after the migration.

## Instructions for contributors

### Keep the root order stable

`src/app/layout.tsx` imports styles in this order:

1. `tokens.css` defines the shared visual vocabulary.
2. `foundations/reset.css` normalizes browser defaults.
3. `app/globals.css` loads owned layout, primitive, pattern, feature, and utility files in their established cascade order.
4. `features/approvals.css` contains the isolated approvals workflow added after the original stylesheet baseline.
5. `layout/responsive-shell.css` applies the consolidated cross-feature viewport and accessibility overrides.

`globals.css` is an import-only manifest. Do not add selectors or declarations to it. Insert a new owned stylesheet at the point in the cascade where its rules are intended to run.

We deliberately did not introduce CSS cascade layers during this remediation. Moving existing selectors into named layers would change precedence rules even when source order appears identical. Native layers can be evaluated later for new, independently isolated components; visual equivalence and low migration risk are the current priority.

### Choose one owner

- `foundations/` owns document-wide defaults and content behavior.
- `layout/` owns the console frame, operator workspace, footer, and cross-feature responsive shell.
- `primitives/` owns reusable controls, states, tables, dialogs, forms, and evidence presentation.
- `patterns/` owns reusable compositions such as filters, record lists, guarded commands, evidence timelines, and detail documents.
- `features/` owns workflow-specific rules. Accounts, transfers, funding, reconciliation, corrections, operations, developer tools, recovery, authentication, and orientation belong here.
- `utilities/` owns narrowly scoped accessibility and typography helpers.

Place a selector with the narrowest truthful owner. A transfer-only form belongs in `features/transfers.css`; a field note shared by several workflows belongs in `primitives/forms.css`. Do not copy a shared selector into multiple feature files to avoid deciding ownership.

### Use semantic visual tokens

Only `tokens.css` may contain literal hexadecimal, RGB, or HSL colors. All other styles must describe intent with a custom property such as `--status-danger-copy`, `--dialog-backdrop`, or `--navigation-active-surface`.

When no existing token expresses the intended role:

1. Add a meaning-based token to `tokens.css`.
2. Reuse it wherever that role recurs.
3. Avoid names tied to a specific numeric shade or one page unless the role is genuinely page-specific.

Spacing, typography, targets, radii, elevations, and z-index values should likewise use existing tokens where they communicate the intended contract.

### Keep responsive behavior explainable

`layout/responsive-shell.css` is the owned cross-feature responsive block. The compact navigation boundary is `760px` and is shared with browser behavior through `src/lib/responsive.ts`; client code must import `COMPACT_NAVIGATION_MEDIA_QUERY` rather than repeat a media-query string.

The consolidated file contains one `760px` compact block and one `520px` narrow block. Feature files may retain a media query only when it controls a self-contained feature and moving it would make ownership less clear. Do not add another cross-feature compact breakpoint.

Reduced-motion and forced-color fallbacks are part of the responsive contract, not optional polish. Any new animation or color-dependent affordance must remain understandable under both preferences.

### Progressive disclosure and rendering

CSS must not hide required evidence merely to reduce visual density. Secondary explanations may be disclosed through existing semantic controls, but amounts, identifiers, timestamps, status, and guarded-action consequences must remain available to assistive technology and keyboard users. Prefer server-renderable markup and CSS for static presentation; introduce a client component only when browser state or interaction genuinely requires it.

## Verification gates

Every style-architecture change must pass:

- lint and production build;
- unit and architecture contract tests;
- the full browser interaction and accessibility suite;
- the platform-native visual regression suite;
- performance and static JavaScript budgets;
- a search proving no non-token color literal was introduced.

The architecture tests additionally ensure the global manifest stays import-only, every imported stylesheet exists, root order remains stable, canonical breakpoint ownership does not drift, and a competing CSS runtime is not added accidentally.

## Migration measurements

| Measurement | Before Phase 12 | Current structure | Interpretation |
| --- | ---: | ---: | --- |
| Source CSS, including the root manifest | 138,064 bytes | 147,725 bytes | The readable split, import manifest, expanded consolidated media blocks, and semantic token names add source text. This is maintainability overhead, not a browser payload measurement. |
| Compiled production CSS | Not captured reliably before the migration | 116,218 bytes | This is the reproducible production-build value used for the current budget. It must not be presented as a before/after reduction without a comparable baseline. |
| Cross-feature `760px` media blocks | Multiple | 1 | Compact navigation and layout rules now have one discoverable owner. |
| Cross-feature `520px` media blocks | Multiple | 1 | Narrow-phone behavior now has one discoverable owner. |
| Non-token literal colors | Distributed across feature and primitive CSS | 0 | Visual roles now pass through semantic tokens. |

The source-size increase is accepted because it buys explicit ownership without changing screenshots, interaction behavior, or the delivered JavaScript architecture. Future optimization decisions should use compiled payload and measured rendering behavior rather than minifiable source bytes.
