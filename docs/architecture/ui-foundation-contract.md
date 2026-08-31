# LedgerSync UI foundation and client-boundary contract

**Phase:** 11  
**Status:** Implemented locally; CI confirmation follows the phase commit  
**Scope:** Shared console display, control, form, and dialog behavior

## Why this contract exists

LedgerSync renders financial evidence. A shared component must therefore preserve exact values, accessible meaning, authorization boundaries, and predictable browser behavior. It must not pull browser-only JavaScript into a server-compatible page merely because an unrelated control needs state.

Before this phase, 35 feature files imported one `features/console/components.tsx` module. That module was marked `use client`, so hook-free headings, badges, state messages, tables, identifiers, timestamps, and evidence blocks all inherited a client boundary. Funding and Corrections also depended on a business-specific dialog component even though their focus and dismissal mechanics were identical.

This phase replaces that graph with direct, narrowly owned imports. There is deliberately no `ui/index.ts` barrel: a server component should be unable to import a client component accidentally through a convenient aggregate module.

## Ownership map

### Server-compatible display modules

These modules contain no `use client` directive and may be rendered by either server or client parents:

- `ui/display/PageHeader.tsx` owns the consistent page introduction and optional page actions.
- `ui/display/StatusBadge.tsx` owns semantic status tones without deciding business state.
- `ui/display/StatePanel.tsx` owns empty, error, offline, denied, and unknown evidence presentation.
- `ui/display/DataTableRegion.tsx` owns a named, keyboard-focusable table region plus optional caption, result summary, and sort explanation.
- `ui/display/RecordLink.tsx` owns the consistent record-navigation affordance.
- `ui/display/Evidence.tsx` owns evidence freshness and definition-list presentation.
- `ui/display/Money.tsx` renders an exact currency/minor-unit pair; it does not parse or calculate money.
- `ui/display/Identifier.tsx` preserves the complete identifier in text and accessible title while CSS may visually constrain it.
- `ui/display/Timestamp.tsx` emits a machine-readable `<time dateTime>` value and an explicit human-readable UTC value.

### Leaf client modules

These modules explicitly declare `use client` because they need browser APIs, state, effects, or element references:

- `ui/controls/CopyControl.client.tsx` performs clipboard interaction and owns its short-lived copied state.
- `ui/controls/FocusedRetry.client.tsx` restores focus after a retry result.
- `ui/controls/Pagination.client.tsx` performs interactive bounded-page navigation.
- `ui/forms/FormField.client.tsx` associates hints, field errors, and an optional error-summary reference with a native form control.
- `ui/overlays/ConfirmationDialog.client.tsx` owns native modal opening, Escape handling, heading focus, busy-state dismissal protection, and trigger-focus restoration.

`ui/controls/Button.tsx` and `ui/controls/IconButton.tsx` remain server-compatible because native button attributes are enough for their base behavior. They become interactive naturally when a client parent supplies an event handler.

## Exact financial evidence rules

### Money

- Callers provide a currency string and a minor-unit integer string.
- Arithmetic, range checks, and conversion stay in `lib/money.ts`.
- The primitive renders the existing reviewed display format and retains `data-currency` and `data-minor-units` machine evidence.
- Invalid or unsupported values use the centralized formatter's fail-closed `Unavailable` result; the component never guesses or uses floating-point arithmetic.

### Identifiers

- The full value remains in the rendered text and `title` attribute.
- Visual truncation must be a CSS concern only. Code must not shorten an account, transfer, event, correction, reconciliation, request, or idempotency identifier.
- Copy behavior is composed separately through `CopyControl`; display-only routes do not need clipboard JavaScript.

### Timestamps

- Valid timestamps render as a semantic `<time>` element.
- `dateTime` is normalized to ISO 8601.
- Visible output includes UTC explicitly through the existing centralized formatter.
- Missing or invalid values render a caller-overridable fallback and never imply a current time.

## State and announcement rules

State panels distinguish evidence states without turning the entire console into a live region:

- Static empty, denied, offline, or unknown explanations have no live role by default.
- A dynamic non-error change opts into `announce="polite"` and becomes `role="status"` with `aria-live="polite"`.
- An error becomes `role="alert"` with assertive announcement.
- The visual kind and announcement behavior are separate decisions. An unknown state is not automatically announced unless it changed dynamically.
- Pages retain responsibility for choosing truthful copy. The primitive never converts unavailable evidence into an empty or successful state.

## Native interaction rules

- Buttons remain native `<button>` elements and forward standard attributes.
- Busy buttons are disabled, expose `aria-busy="true"`, and replace their label with the supplied operation-specific busy text.
- Icon-only buttons require a non-empty accessible label.
- Confirmation dialogs use native `<dialog>` modality. On open, the heading receives focus; Escape is prevented while a financial command is busy; on close, focus returns to the initiating button.
- Business authorization, command copy, exact idempotency behavior, and mutation state remain in the Funding or Corrections feature. The dialog primitive cannot grant permission or submit a request by itself.
- Form fields preserve native inputs while merging existing descriptions with hint, field-error, and summary IDs. Errors set `aria-invalid` without replacing the control.
- `DataTableRegion` describes and frames a native table. It is not a generic grid and does not replace table semantics.

## Migration result

- All 35 former monolithic-component consumers now import the exact primitive they need.
- The client-heavy `features/console/components.tsx` module is removed.
- The business-specific `FinancialCommandDialog.tsx` wrapper is removed; Funding and Corrections compose the generic confirmation dialog with their own policy and copy.
- Displayed financial amounts use `Money`; direct formatter use remains only where an account balance needs one computed accessible label.
- Displayed evidence times use `Timestamp`; the lower-level formatter remains an implementation dependency of the primitive.
- Existing reviewed class names are retained so this architecture change does not silently redesign the product.

## Verification and measured outcome

The audit baseline recorded 32 initial requests. The post-extraction performance journey records 29 initial requests, recovering three request slots and exceeding the two-slot prerequisite for Phase 13.

| Measure | Before Phase 11 | After extraction | Result |
| --- | ---: | ---: | --- |
| Initial requests | 32 audit baseline | 29 | 3 slots recovered |
| Initial API requests | 5 | 5 | unchanged |
| Largest Contentful Paint | 1,304 ms | 1,284 ms | improved by 20 ms |
| Interaction to Next Paint | 24 ms | 24 ms | unchanged |
| Cumulative Layout Shift | 0.001092 | 0.001092 | unchanged |
| Built JavaScript chunks | 31 immediate pre-phase measurement | 32 current budget-script result | within budget |
| Total built JavaScript | 1,198,492 bytes immediate pre-phase | 1,259,654 bytes current budget-script result | increased, investigated and still within the enforced budget |
| Largest JavaScript chunk | 229,156 bytes | 229,156 bytes | unchanged |

The request reduction is the governing Phase 11 result because it creates the route-level headroom required by the plan. The total physical build-artifact byte increase is recorded rather than hidden; it does not cross the repository's enforced JS budgets and will be remeasured after the CSS and route-boundary phases.

Verification covers:

- static modules cannot acquire accidental `use client` directives;
- browser-dependent modules keep explicit leaf client boundaries;
- exact money, identifier, and time machine values;
- intentional state-panel live-region behavior;
- native table and button semantics;
- dialog heading focus, Escape dismissal, trigger-focus restoration, duplicate-submit protection, and financial retry behavior;
- compact, tablet, desktop, forced-colors, visual-regression, accessibility, and performance browser journeys.

## Deliberate non-additions

Storybook is not added in this extraction. The existing page-level Playwright journeys and visual baselines cover the primitives in their real authorization and evidence contexts. A component workbench should be reconsidered only when independently consumed primitives grow enough that page-based review becomes a measurable development bottleneck.

No CSS-in-JS library, utility framework, generic data-grid dependency, dialog package, money package, or date package is introduced. The required semantics are small, inspectable, and already covered by the project's production dependencies and tests.
