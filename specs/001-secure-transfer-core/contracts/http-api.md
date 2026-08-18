# Browser-facing API and Event Contract

The browser calls only same-origin HTTPS BFF routes. The BFF validates session/CSRF and never returns raw internal consistency capabilities.

| Endpoint | Purpose |
|---|---|
| `GET /api/me/accounts` | Authorized owned accounts only. |
| `GET /api/accounts/{id}/balance` | Authorized balance; server applies any valid RYEW requirement. |
| `GET /api/accounts/{id}/transactions?cursor=` | Authorized cursor-paginated history. |
| `POST /api/transfers` | Exact-money transfer; requires `Idempotency-Key`. |

```json
{
  "sourceAccountId": "acc_001",
  "destinationAccountId": "acc_002",
  "amount": { "currency": "USD", "minorUnits": "1250" }
}
```

Success includes `transferId`, final status, exact amount, affected account IDs/balance versions and correlation ID. Errors are stable safe codes: `validation_failed`, `unauthorized_account`, `insufficient_funds`, `idempotency_conflict`, `request_in_progress`, `temporary_unavailable`.

Internal balance-change event fields: `eventId`, `eventType`, `accountId`, `transferId`, `currency`, `availableMinor`, `balanceVersion`, `occurredAt`. Events are at-least-once; a cache never accepts an older version over a newer value.
