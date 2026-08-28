# Data model: new-user UX completion

No new financial database entity is required.

## Existing data used by the UI

| Data | UI use | Rule |
|---|---|---|
| Account | Account setup, funding destination, transfer source/destination | Keep existing authorization and status checks. |
| Funding record | Funding review and approval | Keep the four server-required fields and approval state. |
| Transfer | Transfer review and final result | Keep exact money, idempotency, and immutable result behavior. |
| Reconciliation run | Operational review | Never show a successful result without an authorized response. |
| Operator role/policy | Action availability | Keep server-side authorization as the authority. |

## New UI-only data

| Data | Purpose | Storage |
|---|---|---|
| Field requirement inventory | Records why a field is required or optional | Version-controlled documentation and tests. |
| Plain-language glossary | Explains unavoidable financial terms | Version-controlled Guide content. |

No migration is planned.
