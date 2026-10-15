# Feature Specification: Simple-First LedgerSync

## Goal

Make LedgerSync understandable and calm for business and finance operators without weakening financial truth, authorization, retry safety, or access to technical evidence.

## User Stories

### P1 — Understand the current state

As an operator, I can immediately see whether balances are current, what needs attention, and the safest next action without interpreting infrastructure terminology.

### P1 — Work in Simple or Expert view

As an operator, I start in Simple view and may switch to Expert view to reveal advanced filters, identifiers, timestamps, evidence, developer tools, and diagnostics. Presentation mode never changes authorization.

### P1 — Complete money workflows safely

As an operator, I can create accounts, add money, transfer funds, review approvals, and correct records through clear stages that explain the exact financial effect before confirmation.

### P1 — Recognize uncertainty

As an operator, I am never encouraged to retry an unknown financial outcome. Urgent, stale, mismatched, offline, and uncertain states remain visible in every mode.

### P2 — Find work by urgency

As an operator, I can use a unified Tasks page that orders approval, correction, transfer, balance-check, recovery, and delivery work by urgency and consequence rather than subsystem.

## Functional Requirements

- Simple view is the default and exposes Home, Accounts, Add money, Transfers, Tasks, and Help.
- Expert view exposes existing advanced routes while sharing the same domain and presentation components.
- Experience mode is tenant-and-operator scoped and safely defaults to Simple when unavailable.
- Home presents a trust strip, attention tasks, no more than three primary figures, recent activity, and one suggested next action.
- Status language is produced by centralized presentation adapters.
- Exact money is never abbreviated or converted to floating point.
- Relative times and shortened identifiers reveal exact evidence on request.
- Unavailable actions have visible reasons and recovery guidance; unreleased actions are absent.
- Errors state what happened, what remains true, and the safe next action.
- The responsive shell has no page-level horizontal overflow from 320px upward.
- Sessions are opaque, revocable, PostgreSQL-backed, rotated after authentication, and represented by essential secure cookies.
- Production-capable BFF rate limiting uses shared atomic storage and fails closed when unavailable; local development uses a process-local store.

## Success Criteria

- Four of five representative operators complete every core task without assistance.
- Every participant recognizes an unknown transfer outcome and avoids an unsafe duplicate.
- No Simple-view page exposes unexplained system jargon, raw errors, raw identifiers, or excessive number clusters.
- WCAG AA contrast, keyboard, zoom, forced-colors, reduced-motion, touch-target, and responsive requirements pass.
- Frontend and Go validation suites pass with no unexplained skipped or quarantined failures.
