# Data Model

## Operator UI preference

- Tenant ID + subject ID form the ownership key.
- `experience_mode` is `simple` or `expert`, defaulting to `simple`.
- Preference failure never blocks the product and never changes capabilities.

## Web session

- Hashed random credential, subject, tenant, roles, scopes, authenticated time, expiry, revocation time, and rotation link.
- Consistency requirements are child records keyed by session and account, capped at ten newest entries.
- Raw credentials are returned once, stored only in an HttpOnly cookie, and never logged.

## Presentation status

- Plain title, explanation, attention flag, semantic tone, next action, and optional technical evidence.
- Domain status values remain unchanged and are mapped centrally.

## Task item

- Stable type/id, urgency, title, explanation, amount/account context, safe action, and optional evidence.
- Ordering is urgent unknown outcomes, blocked financial work, approvals, mismatches, then informational work.
