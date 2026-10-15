---
name: LedgerSync
description: A precise operator console for tracing ledger records and their evidence.
colors:
  ink: "#21383e"
  ink-soft: "#354d53"
  muted: "#5e6c72"
  faint: "#5f6d73"
  canvas: "#f1f4f1"
  paper: "#f8faf8"
  paper-subtle: "#edf2ef"
  line: "#d3ddd8"
  line-strong: "#9aaba4"
  rail: "#102a30"
  rail-deep: "#14373e"
  rail-active: "#1a4850"
  rail-accent: "#8ed0b2"
  primary-green: "#13754f"
  primary-green-hover: "#0c6040"
  green-soft: "#e4f1e9"
  success-ink-soft: "#526e63"
  link-blue: "#126b9d"
  amber: "#81500e"
  amber-soft: "#fbf2df"
  red: "#8d392f"
  red-soft: "#f7e9e7"
  focus: "#b27100"
typography:
  display:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif"
    fontSize: "clamp(27px, 2.25vw, 34px)"
    fontWeight: 690
    lineHeight: 1.12
    letterSpacing: "-0.04em"
  headline:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif"
    fontSize: "18px"
    fontWeight: 650
    lineHeight: 1.35
    letterSpacing: "-0.02em"
  body:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: "normal"
  label:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif"
    fontSize: "12px"
    fontWeight: 600
    lineHeight: 1.33
    letterSpacing: "0.01em"
  evidence:
    fontFamily: "SFMono-Regular, Consolas, Liberation Mono, monospace"
    fontSize: "12px"
    fontWeight: 500
    lineHeight: 1.45
    letterSpacing: "normal"
rounded:
  compact: "2px"
  control: "3px"
  surface: "6px"
  pill: "999px"
spacing:
  1: "4px"
  2: "8px"
  3: "12px"
  4: "16px"
  5: "20px"
  6: "24px"
  7: "32px"
  8: "40px"
  9: "48px"
  10: "64px"
components:
  button-primary:
    backgroundColor: "{colors.primary-green}"
    textColor: "#ffffff"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "9px 14px"
    height: "44px"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "9px 14px"
    height: "44px"
  nav-item:
    backgroundColor: "transparent"
    textColor: "{colors.rail-accent}"
    typography: "{typography.label}"
    rounded: "{rounded.compact}"
    padding: "0 12px"
    height: "40px"
  status-chip:
    textColor: "{colors.ink-soft}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "3px 8px"
  search-field:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.compact}"
    padding: "9px 10px"
    height: "38px"
  record-surface:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.compact}"
    padding: "16px"
---

# Design System: LedgerSync

## Overview

**Creative North Star: “Chain of Custody.”**

LedgerSync now carries the selected Chain-of-Custody Register direction across its operator tabs. The shared workspace uses a deep teal navigation rail, a pale mineral canvas, compact utility context, and bordered evidence surfaces. Clear labels, stable alignment, and record identifiers make the interface read like a working register rather than a consumer finance dashboard.

Use the sans-serif family for navigation, headings, and explanations. Reserve monospace for identifiers, timestamps, versions, and exact amounts. Green marks verified or posted evidence and the primary action; amber and red communicate attention and confirmed negative states. Keep color paired with readable wording and iconography.

**Key Characteristics:**
- Evidence-first hierarchy with calm operational density.
- Shared shell, type roles, status language, and document surfaces across tabs.
- Flat, hairline-bordered groupings with deliberate alignment.
- Exact financial facts retain their authority, currency, and time context.

## Colors

The palette combines deep teal structure, mineral paper neutrals, a restrained ledger green, and separate attention colors.

### Primary
- **Ledger Green**: Primary actions and verified or posted evidence. Use the darker green for hover and emphasis.
- **Register Teal**: The navigation rail and active navigation state anchor the shared operator workspace.

### Secondary
- **Record Blue**: Text links and secondary record navigation; it does not communicate success.

### Neutral
- **Mineral Canvas**: The page background keeps the workspace soft and low-glare.
- **Record Paper**: Evidence documents, tables, and grouped content.
- **Subtle Paper**: Secondary groupings and muted control surfaces.
- **Ink**: Primary headings and record text.
- **Soft Ink**: Supporting labels and secondary values.
- **Quiet Metadata**: Timestamps and explanatory metadata where lower emphasis is appropriate.
- **Hairline**: Default dividers; the stronger line marks important boundaries and controls.

### Named Rules
**The One Meaning Per Color Rule.** Green means verified or posted; amber means attention; red means a confirmed rejection, error, or blocked action. Links remain blue and never borrow success color to look interactive.

## Typography

**Display Font:** Segoe UI Variable Text/system sans stack.
**Body Font:** Segoe UI Variable Text/system sans stack.
**Label/Mono Font:** SFMono-Regular, Consolas, Liberation Mono, monospace for immutable evidence.

**Character:** The sans family keeps dense operational pages direct and readable. Monospace appears as a narrow evidence accent for values that operators compare or copy.

### Hierarchy
- **Display**: Restrained, firm page titles; used once per page.
- **Headline**: Compact panel and section titles.
- **Body**: Default explanatory and row text; keep paragraphs readable and short.
- **Label**: Field names, table headers, and compact controls.
- **Evidence**: Identifiers, timestamps, versions, and aligned exact amounts; use tabular numerals for comparisons.

### Named Rules
**The Evidence Mono Rule.** Monospace is for immutable identifiers and numeric evidence, never whole explanatory paragraphs or page titles.

Use 12px as the minimum text size, including navigation context, status labels, filters, identifiers, and timestamps. Text colors must reach at least 4.5:1 contrast against their assigned surfaces. Keep verified time and historical status visible, and allow supporting evidence text to wrap.

## Layout

The shared shell keeps tenant, environment, operator, and navigation context in a persistent desktop rail and utility bar. The workspace uses a fluid twelve-column grid with a bounded wide-screen measure. Two-column evidence layouts are reserved for content that benefits from a main record and related context; other screens use the available measure without stretching individual controls.

At tablet widths the rail narrows and data regions keep their own scroll behavior. At the compact breakpoint, navigation becomes a labelled drawer behind a compact top bar, content gutters tighten, and multi-column evidence stacks in document order. Tables retain semantic columns inside a horizontal scroll region; preserve the identifying column where the view supports it. Keep labels, statuses, currency, exact amount, and evidence timestamps visible at every width.

The spacing rhythm follows the shared four-pixel base scale. Use compact intervals within dense record groups and larger intervals to distinguish major work areas.

## Elevation & Depth

The default workspace is flat. Thin borders, spacing, background tone, and alignment establish document boundaries; ordinary evidence does not float above the page. Shadows are limited to overlays such as the mobile navigation drawer, dialogs, and positioned filter panels.

### Named Rules
**The Flat Record Rule.** Keep ledger and evidence surfaces at rest without decorative shadow. Reserve depth for layers that actually cover or leave the document flow.

## Shapes

The form language is squared and document-like. Shared record surfaces use compact corners and hairline borders; controls have a small radius, while the generic status chip retains its pill silhouette. Some dense tables tighten status shapes locally. Avoid large rounded cards and decorative capsules for primary actions.

## Components

### Buttons
- **Character:** Compact controls with explicit text and a clear action hierarchy.
- **Shape:** Slightly softened corners (3px).
- **Primary:** Ledger-green fill with white text; compact horizontal padding and a 44px minimum target.
- **Hover / Focus:** Darker green on hover; keyboard focus uses the shared high-contrast outline.
- **Secondary:** Transparent or paper surface with a visible boundary and ink text.
- **Danger:** Restrained red is reserved for confirmed destructive or negative actions.

### Chips
- **Style:** Compact outlined status label with an icon and readable status text.
- **State:** Success, warning, danger, neutral, and informational tones use the shared semantic palette. Some tight register tables use a square-corner variant.

### Cards / Containers
- **Corner Style:** Compact squared record edges; larger surfaces may use the base surface radius where already present.
- **Background:** Paper over mineral canvas, separated by a fine border.
- **Shadow Strategy:** Flat at rest; see Elevation & Depth.
- **Internal Padding:** Follow the shared spacing scale and reduce padding only inside dense evidence rows.

### Inputs / Fields
- **Style:** Visible label, paper fill, ink text, and a fine boundary. Keep field sizing and validation appropriate to the specific task.
- **Focus:** Use the shared focus outline; do not rely on color changes alone.
- **Error / Disabled:** Explain the state in text and preserve relevant entered context.

### Navigation
- **Style:** Deep teal vertical rail on desktop, compact labelled drawer on small screens.
- **Default / Active:** Muted rail text at rest; active items gain a lighter teal field and a pale green marker.
- **Context:** Keep tenant, environment, operator, and active location legible in the shell.

### Data Tables
- **Style:** Native table inside a bounded horizontal scroll region, with light header tint and fine row dividers.
- **Alignment:** Use monospace tabular numerals for exact amounts and align numeric columns consistently.
- **Selection:** A quiet green-tinted row and edge marker identify the active record; retain textual and keyboard-accessible selection semantics.

### Evidence Status
- **Style:** Pair status wording with its semantic color and icon. Keep posting and downstream delivery as separate facts.
- **Freshness:** Show the verified time or a clear unavailable/historical state. Never style missing evidence as a current zero.

## Do's and Don'ts

### Do:
- **Do** use the shared teal rail, mineral canvas, paper surfaces, type roles, and semantic colors across operator tabs.
- **Do** use monospace tabular figures for exact amounts, IDs, timestamps, and versions.
- **Do** keep focus visible, maintain semantic table markup, and preserve labels at compact widths.
- **Do** display financial evidence only from authorized, source-backed records; clearly label stale or unavailable values.
- **Do** name the current control “internal reconciliation” and explain once that it compares ledger postings with stored balances. Do not imply bank or provider settlement was checked.
- **Do** keep one decision-relevant explanation and one next action in contextual rails; remove repeated instructional cards.
- **Do** treat PostgreSQL as the sole authority for customer-visible balances; the disposable cache never proves that money changed.
- **Do** preserve exact minor-unit amounts, currency precision, and the distinction between posting and delivery states.
- **Do** gate financial actions by server-issued authority and retain deliberate, auditable recovery behavior.

### Don't:
- **Don't** invent balances, transfers, reconciliation results, customers, or activity for a live view.
- **Don't** imply a cached balance is current or treat unavailable evidence as zero.
- **Don't** merge posting status with asynchronous delivery status.
- **Don't** remove authorization, idempotency, audit, or confirmation behavior when changing layout or styling.
- **Don't** use gradients, glass effects, decorative financial imagery, oversized rounded cards, or color alone to carry meaning.
