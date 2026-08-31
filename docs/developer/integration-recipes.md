# Partner integration recipes

These recipes apply to LedgerSync's private API. In the local workspace, use
the same-origin BFF and the protected local development identity. In a managed
environment, obtain a client-credentials token from the approved identity
provider and send it only to the private API edge. Never copy browser cookies,
BFF actor assertions, or secret-manager values into a partner service.

## Common rules

- Amounts and versions are JSON strings. For INR, `"12550"` means 125.50 INR
  when an endpoint uses minor units; transfer input uses the documented exact
  decimal format.
- Every write uses a new opaque `Idempotency-Key` (16–255 characters). If a
  response is lost, retry the exact same request with the exact same key.
- Use `X-Request-ID` and the returned resource identifier when asking support
  to investigate an outcome.
- Never treat a webhook receipt as financial settlement; webhook delivery is
  an at-least-once notification channel.

## curl — create an exact transfer

```bash
curl --fail-with-body --request POST "$LEDGERSYNC_API_URL/transfers" \
  --header "Authorization: Bearer $LEDGERSYNC_ACCESS_TOKEN" \
  --header "Content-Type: application/json" \
  --header "Idempotency-Key: transfer-20260829-001" \
  --data '{
    "source_account_id":"70000000-0000-4000-8000-000000000001",
    "destination_account_id":"70000000-0000-4000-8000-000000000002",
    "amount":"125.50",
    "currency":"INR"
  }'
```

If the connection fails after sending this request, repeat the identical
command. Do not create a replacement key until the original outcome is known.

## TypeScript — retry safely

```ts
const key = crypto.randomUUID();
const payload = {
  source_account_id: sourceAccountId,
  destination_account_id: destinationAccountId,
  amount: "125.50",
  currency: "INR",
};

const response = await fetch(`${baseUrl}/transfers`, {
  method: "POST",
  headers: {
    Authorization: `Bearer ${accessToken}`,
    "Content-Type": "application/json",
    "Idempotency-Key": key,
  },
  body: JSON.stringify(payload),
});
// Persist key + payload until the result is known. Retry the same pair after
// a timeout; do not parse money into a JavaScript number.
```

## Go — preserve the original idempotency key

```go
request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/transfers", bytes.NewReader(body))
if err != nil { return err }
request.Header.Set("Authorization", "Bearer "+accessToken)
request.Header.Set("Content-Type", "application/json")
request.Header.Set("Idempotency-Key", idempotencyKey) // persisted before send
response, err := client.Do(request)
// If err is a timeout after send, submit the identical body and same key.
```

## Postman

Import `contracts/generated/ledgersync.postman_collection.json`. Set
`baseUrl`, `bearerToken`, and `actorAssertion` only in your local/private
environment. The collection is an operation catalogue; it never contains
working credentials or a request runner.

## Webhook endpoint ownership and delivery

1. Create a signing key in the approved secret manager. Register only its
   non-secret reference and key ID with `POST /developer/webhooks`.
2. LedgerSync returns `pending_verification`; it does not return a challenge.
3. LedgerSync's worker POSTs an expiring challenge to the HTTPS endpoint using
   a signed request. The endpoint must return HTTP 2xx and
   `X-LedgerSync-Verification: v1=<hex HMAC-SHA-256(challenge)>` using the
   registered signing key.
4. Poll the webhook record until it is `active`. A bad proof, expired
   challenge, or exhausted retry is not activation.
5. Deduplicate delivered events by `X-LedgerSync-Event-ID`. Verify the
   timestamp, key ID, and HMAC before using an event. Acknowledge quickly,
   process asynchronously, and use the delivery history plus approved replay
   flow for recovery.

## Contract source

The authoritative private API is [OpenAPI](../../contracts/openapi.yaml).
Generated TypeScript, Go, and Postman catalogues must be regenerated from that
file; do not hand-edit generated outputs.
