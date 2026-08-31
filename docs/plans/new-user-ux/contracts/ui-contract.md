# UI contract: field requirements and plain language

## Field label contract

| Field state | Visible label | Validation behavior |
|---|---|---|
| Required | `<Field name> Required` | The UI and server reject a missing value. |
| Optional | `<Field name> Optional` | The server accepts a missing value. |
| Search/filter | No required marker unless the server requires it | Empty value means no filter or default range. |

## Error contract

Errors must state the next user action. They must not expose internal infrastructure, secret values, or unsupported success claims.

Examples:

- `Choose an account to continue.`
- `Enter an amount greater than zero.`
- `Add the reference number from the payment record.`
- `We could not confirm whether the request was saved. Retry with the same request.`

## Progress disclosure contract

- The dashboard starts with the next safe action.
- Advanced operational detail is shown after the primary action or inside a clearly named disclosure.
- Financial facts, IDs, amounts, currencies, status, and approval requirements stay visible when they affect a decision.
