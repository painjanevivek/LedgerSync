# Pilot security, limits, and currency controls

LedgerSync fails closed for financial mutations and keeps read availability where a disposable rate-control write fails. The API enforces bounded headers and request lifetimes, PostgreSQL-backed tenant/principal/route rate windows, and transactional transfer policy.

## Before enabling partner traffic

1. Record the finance/product-approved pilot currency and limits in the change ticket. `USD` in local fixtures is demonstration-only.
2. Set `LEDGERSYNC_PILOT_CURRENCY` and the bounded HTTP/rate environment variables. Production startup refuses tenant policies, accounts, or transfers outside the selected currency.
3. Provision exactly one `tenant_transfer_policies` row and explicit `account_credit_permissions` relationships for each permitted actor/destination. Same-tenant existence alone never authorizes a credit.
4. Apply `deploy/postgres/roles.sql` as database owner, then grant each NOLOGIN group role to a separately authenticated workload identity. Do not grant standing authority to `ledgersync_break_glass`.
5. Run the live integration suite and archive its amount/velocity concurrency, rate-limit, immutable-row, destination-authorization, and OpenAPI evidence.

## Rate-limit response

- `429 rate_limited` includes integer `Retry-After` seconds.
- Transfer rate-state failure is fail-closed. Read rate-state failure is fail-open so an operational limiter outage does not hide authoritative balances or evidence.
- Rate windows contain only a SHA-256 principal digest, tenant ID, route key, window, and count. They are disposable and must never be treated as audit or financial evidence.

## Timeout outcome

- A BFF read timeout returns `504 upstream_timeout` and can be retried normally.
- A BFF transfer timeout returns `504 transfer_outcome_unknown`. Retry the identical request with the same `Idempotency-Key`; never generate a new key until the original result is known.

## Policy denial and investigation

Amount, currency, destination, and rolling-velocity decisions occur inside the serializable transfer transaction. Policy denials create a redacted `transfer.policy_denied` audit event without storing amount, balance, credentials, or raw request payload. A required audit-write failure keeps the mutation denied and raises an internal operational error.
