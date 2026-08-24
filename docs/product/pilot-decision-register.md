# LedgerSync pilot decision register

**Status:** Conditional approval for local implementation; shared production remains gated  
**Recorded:** 2026-08-23  
**Applies to:** Functional and responsive MVP completion plan, Phases 0–12

This register separates implementation assumptions from decisions that require product, finance, legal, security, or operations approval. Engineering may build and verify the isolated local product using the assumptions below. Nothing in this document represents legal advice, regulatory approval, custody authorization, or production sign-off.

## Approved technical direction

| Topic | Working decision | Why |
|---|---|---|
| Product boundary | API-first, closed-loop, same-currency LedgerSync accounts | Proves the ledger core without adding external payment rails |
| Financial authority | PostgreSQL ledger and projections | Transactions, reconciliation, backup, and recovery require one durable source of truth |
| Fast reads | Versioned disposable Redis cache with primary fallback | Performance must not weaken correctness |
| Money representation | Currency plus integer minor units encoded as strings at JSON boundaries | Avoids floating-point and JavaScript precision errors |
| Dashboard role | Investigation first; transfer creation requires explicit `transfers:write` scope and tenant policy | Preserves least privilege and the API-first product boundary |
| Local product demonstration | Server-controlled demo session, deterministic PostgreSQL fixtures, real BFF/API routes | Enables real interaction without weakening production OIDC |
| Responsive implementation | One semantic component tree and shared view models for mobile, tablet, laptop, and desktop | Prevents financial facts, permissions, and states from drifting by device |
| Reconciliation language | A passing result requires an authoritative completed run with zero mismatches | The UI must never claim evidence it did not receive |
| Delivery language | Financial posting state and webhook/notification delivery state are separate | A delayed notification does not make committed money pending |

## Local implementation assumptions

These assumptions support deterministic development and automated testing. They are not production approvals.

- Demo and India pilot currency: INR, represented in paise.
- Demo jurisdiction: intentionally unspecified and labeled non-production.
- Custody posture: non-custodial ledger infrastructure; no bank/card/FX/regulated-funds claims.
- Demo tenant: one isolated tenant with explicitly scoped operator identities.
- Dashboard transfers: enabled only for the isolated demo writer and test identities.
- Idempotency retention target: at least 30 days.
- Reconciliation target: hourly when affordable, never less than daily for a pilot environment.
- Recovery proposal: RPO at most 5 minutes and RTO at most 60 minutes, pending operations approval.
- Browser support proposal: current and previous Chrome/Edge, current Firefox, current Safari/iOS Safari.

## Production gates requiring named approval

| Gate | Required decision/evidence | Owner | Deadline | Blocking effect |
|---|---|---|---|---|
| Jurisdiction | Name one production jurisdiction and applicable obligations | Product + legal | Before partner contract | Blocks shared production |
| Pilot currency | Formally approve the one pilot currency and precision policy | Product + finance | Before API contract sign-off | Blocks partner contract freeze |
| Custody/regulatory boundary | Confirm non-custodial posture or name licensed partner and responsibilities | Legal + product | Before production data | Blocks shared production |
| Balance semantics | Define posted/available/ledger balance and permitted aggregation categories | Finance + product | Before overview release | Blocks aggregate balance UI |
| Dashboard write policy | Approve tenants, roles, limits, and workflow allowed to post from the console | Product + security/risk | Before production UI enablement | Defaults console to read-only |
| Transfer limits | Approve amount, velocity, and tenant/role limits | Risk + product | Before production transfer enablement | Blocks console/API production writes |
| Reconciliation | Approve cadence, evidence format, retention, and mismatch ownership | Finance + operations | Before pilot traffic | Blocks pilot graduation |
| Recovery | Approve RPO/RTO, retention, restore owner, and drill cadence | Operations + product | Before pilot traffic | Blocks pilot graduation |
| Managed identity/secrets/PITR | Configure selected provider and preserve test evidence | Security + platform | Before shared production | Blocks shared production |

## Explicit non-goals

- Bank rails, cards, FX, custody, external settlement, chargebacks, holds, or scheduled transfers.
- Public consumer wallet or native iOS/Android applications.
- Custom password or token-lifecycle implementation.
- Editable/deletable posted ledger history.
- Client-side financial authority or offline transfer queue.
- Kubernetes, service mesh, microfrontends, active-active multi-region writes, or event sourcing as the sole source of financial truth.

## Decision-change protocol

Any change that expands movement scope, custody, currencies, authorization, transfer lifecycle, or financial authority requires:

1. an updated PRD/specification;
2. architecture and data-model review;
3. threat/risk assessment;
4. migration and compatibility plan;
5. updated contract and test evidence;
6. approval by the relevant owner named above.
