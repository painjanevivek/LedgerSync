# Console field inventory

**Checked:** 2026-08-29
**Rule:** A field is `Required` only when the command cannot be accepted without it. A search or filter is `Optional`.

## Accounts

| Screen | Field | Status | Evidence | User help needed |
|---|---|---|---|---|
| Create account | Display name | Required | Account creation validation requires 1–120 non-control characters. | Explain that this is the name shown in LedgerSync. |
| Create account | External reference | Required | Account creation validation requires a 3–64 character reference. | Explain that this is the user’s own stable reference. |
| Create account | Category | Required | The API requires one allowed category; the form has a default. | Explain what the category is used for. |
| Create account | Currency | System-set | The local flow sends only INR. The user cannot edit it. | Explain that local accounts use INR. |
| Account lifecycle | Reason | Required | Status-change requests require a non-empty reason. | Explain that the reason records why the change is being made. |
| Close account | Confirm external reference | Required for closure | Closing requires the exact external reference. | Explain that closure is final. |
| Accounts list | Search accounts | Optional | Empty search means no text filter. | Give one short example. |
| Accounts list | Status | Optional | Empty status means all statuses. | No extra help needed. |
| Accounts list | Category | Optional | Empty category means all categories. | No extra help needed. |

## Funding

| Screen | Field | Status | Evidence | User help needed |
|---|---|---|---|---|
| Add funding record | Account | Required | Funding creation requires a destination account. | Already present. |
| Add funding record | Amount | Required | Funding creation requires a positive exact amount. | Already present. |
| Add funding record | Reference number | Required | Funding creation requires an external reference. | Already present. |
| Add funding record | Supporting document | Required | Funding creation requires an evidence reference. | Already present. |
| Funding approval or rejection | Reason | Required | A decision is disabled without a reason. | Explain that it records what was checked. |
| Funding compensation | Reason | Required choice | The request has one selected reason code. | Give plain choices. |
| Funding compensation | Operator note | Required | The request is disabled without a note. | Explain what evidence supports the correction. |

## Transfers

| Screen | Field | Status | Evidence | User help needed |
|---|---|---|---|---|
| Prepare transfer | From account | Required | A transfer requires a valid source account. | Explain that money leaves this account. |
| Prepare transfer | To account | Required | A transfer requires a different valid destination account. | Explain that money goes to this account. |
| Prepare transfer | Amount | Required | Exact-money parsing rejects an empty or invalid amount. | Explain INR example and no rounding. |
| Transfers list | Search transfers | Optional | Empty search means no text filter. | Give one short example. |
| Transfers list | Financial status | Optional | `All statuses` is a valid no-filter state. | No extra help needed. |

## Corrections

| Screen | Field | Status | Evidence | User help needed |
|---|---|---|---|---|
| Start correction request | Reason | Required choice | The correction API accepts only an approved reason code. | Use plain reason names. |
| Start correction request | Verified operator note | Required | The correction API rejects an empty note. | Explain the note must say why the correction is needed. |
| Corrections list | Status | Optional | `All` is a valid no-filter state. | No extra help needed. |
| Approve, reject, or cancel correction | Decision or cancellation reason | Required when shown | The action is disabled without a reason. | Explain that it records the decision. |

## Events

| Screen | Field | Status | Evidence | User help needed |
|---|---|---|---|---|
| Events list | Event type | Optional | Empty value means no event-type filter. | No extra help needed. |
| Events list | State | Optional | Empty state means all delivery states. | No extra help needed. |
| Events list | Related ID | Optional | Empty value means no related-record filter. | Explain that it can be an account or transfer ID. |
| Events list | Correlation ID | Optional | Empty value means no correlation filter. | Explain that it links related system activity. |
| Events list | From UTC | Optional | Empty value means no start-time filter. | Provide the ISO UTC example. |
| Events list | To UTC | Optional | Empty value means no end-time filter. | Provide the ISO UTC example. |

## Rules for implementation

- Do not add `Optional` to the system-set Currency field.
- Use native `required` for user-entered required fields.
- Keep a valid default selected for required choice fields, and label the choice as `Required`.
- Required labels and disabled submit buttons must agree with server validation.
- A user-facing label must never claim a field is optional when the server will reject it.
