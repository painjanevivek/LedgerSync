# Implementation Plan: Simple-First LedgerSync

## Architecture

- Keep Go/PostgreSQL authoritative for financial data, permissions, and server sessions.
- Add a presentation layer that maps domain states to plain-language summaries and optional expert evidence.
- Implement Simple and Expert modes as presentation density/disclosure variants over shared components.
- Persist the mode through the existing operator-preference boundary; presentation mode never grants capability.
- Use semantic CSS tokens and cascade layers, with feature-owned responsive rules.

## Delivery

1. Integrate validated responsive, typography, rate-limit, and availability work.
2. Add the presentation model, preference provider, redesigned shell, trust strip, and navigation.
3. Redesign Home and add unified Tasks.
4. Apply simple-first language and evidence disclosure to money workflows and expert tools.
5. Add opaque server sessions and exact cookie lifecycle.
6. Decompose hotspots, finish responsive/accessibility work, and validate.

## Governance

- Preserve integer minor-unit money, idempotency, CSRF, tenant isolation, and unknown-outcome behavior.
- Add forward-only migrations and ADRs for session authority and frontend protection.
- Do not provision cloud resources, write secrets, deploy, or push.
