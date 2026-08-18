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

**Transfer transition**: validate identity and request → reserve/resolve idempotency → lock account projections in stable order → verify funds → create journal/postings/projections/outbox/stable response atomically → publish asynchronously. A correction is a new compensating journal transaction.
