# Tenant role, scope, and transfer-limit policy — approval draft

Status: engineering proposal; product, security, and risk approval is required
before production writes. Local demo values are not production defaults.

## Least-privilege role proposal

| Role | Allowed API scopes | Account relationship | Explicitly excluded |
|---|---|---|---|
| Viewer | `accounts:read`, `transactions:read`, `transfers:read`, `reconciliation:read` | Read only on explicitly mapped accounts | Transfer creation, recovery actions, provisioning |
| Operator | Viewer scopes plus `transfers:write` | Read + debit on source and credit permission on destination | Policy changes, recovery replay, credentials |
| Finance reviewer | Viewer scopes | Read only; complete tenant reconciliation evidence | Transfer creation by default, provisioning, recovery execution |
| Support investigator | Minimum case-specific read scopes | Read only on authorized tenant/account | Transfer creation, raw secrets, unrestricted cross-tenant search |
| Tenant administrator | Identity/subject/account administration through the internal provisioning workflow | No implicit money movement permission | Standing break-glass or recovery replay |
| Recovery approver | No ordinary transfer scope required | Time-bound case scope | Single-person approve-and-execute; financial row mutation |

Roles are descriptive policy labels; API authorization remains scope plus
tenant/account relationships. A role must never grant account access implicitly.
OIDC claims are filtered by the server allowlist. `platform:root`, wildcard
scopes, and unknown roles/scopes are rejected.

## Transfer policy decisions required per tenant

The provisioning record requires canonical integer minor-unit strings for:

- minimum and maximum amount per transfer;
- actor rolling 24-hour total;
- source-account rolling 24-hour total;
- tenant rolling 24-hour total;
- the single pilot currency;
- explicit debit subjects and destination-credit subjects.

Engineering intentionally supplies no production numbers. Risk must provide the
approved values and rationale for each partner. The hierarchy must keep the
maximum transfer at or below actor/source limits and both actor/source limits at
or below the tenant limit. Missing policy, inactive/frozen/closed accounts,
unapproved destination, currency mismatch, or exceeded limit fails closed inside
the same serializable transaction as the transfer decision.

## Privileged action controls

- Transfer confirmation shows exact source, destination, amount, currency, and
  retry semantics; an unknown outcome retains the original idempotency key.
- Dead work replay uses inspect → separate approval → execute, with durable audit
  evidence and no ability to mutate ledger postings.
- Provisioning is an internal CLI/workflow, not public self-service, and records
  credential references rather than secret material.
- Break-glass database authority is NOLOGIN by default, time bound, separately
  approved, monitored, and revoked after use.

## Approval record

| Owner | Required decision | Name | UTC date | Evidence/ticket | Status |
|---|---|---|---|---|---|
| Product | Console write roles and confirmation policy | — | — | — | Pending |
| Security | Scope mapping, separation of duties, break glass | — | — | — | Pending |
| Risk | Per-transfer and velocity values | — | — | — | Pending |
| Finance | Account-category and balance semantics dependency | — | — | — | Pending |

Production `transfers:write` remains disabled for partner subjects until all four
rows are approved and the configured policy is exercised by authorization and
concurrency tests.
