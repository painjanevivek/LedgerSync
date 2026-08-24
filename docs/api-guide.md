# API integration guide

The production integration point is the private API behind an authenticated BFF
or trusted service boundary. Do not expose the private API directly to public
browsers.

## Create an internal transfer

`POST /api/transfers` requires an `Idempotency-Key`, trusted caller
authorization, a verified actor assertion, and canonical decimal text parsed
server-side into integer minor units.

```json
{
  "source_account_id": "source-account-uuid",
  "destination_account_id": "destination-account-uuid",
  "amount": "12.50",
  "currency": "INR"
}
```

| Outcome | Caller action |
|---|---|
| `201` posted | Store transfer ID and the returned consistency requirements. |
| replayed final response | Treat it as the original result; do not create another transfer. |
| `409 insufficient_funds` | No money moved. Ask the user to adjust the amount. |
| `409 idempotency_conflict` | The key was reused with different intent; create a genuinely new request only after user confirmation. |
| network / `503` | Retry the **same body and key** until final state is known. |

Never use JavaScript, Go, or database floating-point values for money. Exact
amounts are integer minor units and currencies are explicit uppercase codes.
