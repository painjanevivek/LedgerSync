# Browser-facing API and Event Contract

The browser calls only same-origin HTTPS BFF routes. The BFF validates session/CSRF and never returns raw internal consistency capabilities.

| Endpoint | Purpose |
|---|---|
| `GET /api/me/accounts` | Authorized owned accounts only. |
| `GET /api/accounts/{id}/balance` | Authorized balance; server applies any valid RYEW requirement. |
| `GET /api/accounts/{id}/transactions?cursor=` | Authorized cursor-paginated history. |
| `POST /api/transfers` | Exact-money transfer; requires `Idempotency-Key`. |
| `GET /api/transfers?cursor=&accountId=&status=&from=&to=` | Tenant-authorized, cursor-paginated transfer investigation list. |
| `GET /api/transfers/{id}` | Immutable transfer facts, journal/postings, audit timeline, and separate delivery state. |
| `GET /api/reconciliation/runs?cursor=` | Authorized reconciliation run history; never synthesizes passing evidence. |
| `GET /api/reconciliation/runs/{id}` | Run scope, watermark/version, counts, result, mismatches, timestamps, and immutable audit reference. |

```json
{
  "sourceAccountId": "acc_001",
  "destinationAccountId": "acc_002",
  "amount": { "currency": "USD", "minorUnits": "1250" }
}
```

Success includes `transferId`, final status, exact amount, affected account IDs/balance versions and correlation ID. Errors are stable safe codes: `validation_failed`, `unauthorized_account`, `insufficient_funds`, `idempotency_conflict`, `request_in_progress`, `temporary_unavailable`.

Internal balance-change event fields: `eventId`, `eventType`, `accountId`, `transferId`, `currency`, `availableMinor`, `balanceVersion`, `occurredAt`. Events are at-least-once; a cache never accepts an older version over a newer value.

List/detail responses use stable object IDs and cursor pagination. Money remains string minor units plus currency. Transfer financial status (`posted` or `rejected` for the pilot) is never overloaded with webhook/notification delivery status. Reconciliation responses return `evidence_unavailable` when authoritative evidence cannot be obtained; the BFF must not replace this with a successful placeholder.
