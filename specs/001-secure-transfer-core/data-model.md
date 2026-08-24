# Secure Transfer Core — Data Model

## Principles

- Amounts are positive exact minor units; debit/credit direction is separate.
- Ledger postings are immutable. Projections are updateable/rebuildable speed structures.
- Tenant/user identifiers establish isolation and object-level authorization.

| Entity | Required fields | Rules |
|---|---|---|
| Account | ID, tenant, currency, status, kind, timestamps | Currency is immutable; only active accounts transfer. |
| Account ownership | account ID, user ID, role, grant/revoke times | Determines view/debit authority. |
| Balance projection | account ID, available/ledger minor values, version, updated time | Version increases with each balance change. |
| Transfer/journal | transfer ID, actor, source/destination, amount/currency, status, timestamps | Accounts differ and share currency. |
| Ledger posting | ID, journal ID, account ID, debit/credit, positive minor value, currency | Transaction balances to zero per currency; no edits/deletes. |
| Idempotency request | tenant/actor/operation/key, fingerprint, state, response, expiry | Unique per actor/operation/key; mismatch is conflict. |
| Outbox event | event ID, account, version, payload, attempts, due/published time | Durable, repeat-safe publication obligation. |
| Consistency requirement | user/account/minimum version, issue/expiry, audience/key metadata | Short-lived; server-side; never authorization alone. |
| Audit event | actor/target/type/outcome/correlation/sanitized metadata | No secrets, tokens, PII or raw balances. |
| Reconciliation run | ID, tenant, scope, ledger watermark/version, code/schema version, accounts/postings checked, mismatch count, result, start/end time, audit reference | Immutable evidence; “passed” is valid only when the completed run has zero mismatches. |
| Reconciliation mismatch | ID, run ID, affected account/scope, expected/observed or sanitized difference, investigation status, owner, resolution/compensating journal link, timestamps | Never resolved by editing ledger history; resolution remains auditable. |
| Delivery attempt | ID, transfer/event, endpoint, attempt number, status, response class, due/started/completed time, sanitized error | Delivery state is separate from the transfer’s committed financial state. |

Development/demo fixtures use the same entities and migrations but live only in an isolated non-production database. Demo identity and fixture provenance are configuration/runtime concerns, not financial-domain entities, and production startup rejects demo mode.

**Transfer transition**: validate identity and request → reserve/resolve idempotency → lock account projections in stable order → verify funds → create journal/postings/projections/outbox/stable response atomically → publish asynchronously. A correction is a new compensating journal transaction.
