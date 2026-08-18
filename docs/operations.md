# Operator and pilot operations guide

## Operator console

The console is intentionally small: it shows only authorized accounts, an
authoritative balance state, immutable transfer history, and a guarded
same-currency transfer form. If a balance cannot meet the user’s
read-your-writes requirement, it says **Temporarily unavailable** instead of
presenting an old number as current.

When a transfer result is not confirmed, use **Retry same transfer**. The UI
retains the idempotency key for that in-progress request and sends it again.

## Pilot release ownership

Before a shared pilot, the accountable operator must record:

1. Pilot jurisdiction and single supported currency.
2. Named OIDC provider configuration using authorization code with PKCE.
3. Managed PostgreSQL backup age and isolated restore drill evidence.
4. Reconciliation result (`0` mismatches) and RYEW violation count (`0`).
5. On-call ownership for database, Redis/outbox, identity provider, and design
   partner support.

Use the runbooks in `docs/runbooks/` for incidents. Stop pilot expansion for a
reconciliation mismatch, duplicate movement, authorization disclosure, or a
balance that was shown current without meeting its required version.
