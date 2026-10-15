# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Inferred from `DESIGN.md` and the application routes: finance and operations teams investigating accounts, transfers, reconciliation, and delivery issues; partner engineers integrating the API and inspecting related evidence.

## Product Purpose

Inferred from `DESIGN.md`: LedgerSync is an API-first, closed-loop ledger platform. The operator console helps people verify financial records, trace decisions, and recover safely under pressure. Success means an operator can explain what happened, whether the ledger record is complete and balanced, which balance is authoritative, and what safe action comes next.

The current reconciliation checks LedgerSync postings against its stored balance projections. It does not ingest bank or payment-provider statements. Funding evidence does not prove external settlement.

## Positioning

Inferred from the existing product brief and code: immutable ledger records and authoritative balance evidence are central to the product; asynchronous delivery state is shown separately from the financial posting outcome.

## Operating Context

Inferred from `DESIGN.md` and route structure: the web console supports account, transfer, reconciliation, funding, approval, webhook, event, recovery, and audit workflows. Operators work across tenant and environment context, and may investigate under time pressure.

## Capabilities and Constraints

Inferred from `DESIGN.md`: production pilot scope is 2–3 design partners, one jurisdiction, one currency, and internal same-currency transfers. Preserve existing authorization, financial command, idempotency, audit, loading/error/offline, and recovery behavior. Never invent financial evidence or present stale/cached values as authoritative.

Current priority: finish controlled financial database permissions and enforce tenant isolation. External reconciliation is a customer-validation hypothesis: one read-only source, a mapped clearing account, deterministic matches, and reviewed exceptions if demand is established. Payouts, cards, FX, multi-provider orchestration, and automatic variance adjustments are deferred.

## Brand Commitments

Inferred from `DESIGN.md`: retain the LedgerSync name and the product’s high-trust operational character. The current brief describes a calm, precise interface rather than a consumer wallet.

## Evidence on Hand

The repository contains product and design briefs, an existing implementation, and visual references under `docs/design/`. Do not fabricate customers, testimonials, balances, reconciliation results, or product performance claims.

## Product Principles

Inferred from the existing brief:

- Evidence must explain financial facts.
- Failure states must distinguish what is known from what remains unknown.
- Recovery actions must be safe and deliberate.
- Tenant, environment, and authorization context must remain clear.
- Accessibility and responsive operation are correctness requirements.

## Accessibility & Inclusion

Inferred from `DESIGN.md`: target WCAG 2.2 AA, preserve keyboard and screen-reader operation, support zoom and text spacing, respect reduced motion, and keep financial context and key actions available on mobile.
