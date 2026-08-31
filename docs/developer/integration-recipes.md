# Partner integration recipes

These recipes describe LedgerSync Private API contract `2.0.0`. The
authoritative operation, schema, error, pagination and response-header source
is [OpenAPI](../../contracts/openapi.yaml). Generated catalogues must converge
with that file; this guide explains how to combine those operations safely.

LedgerSync records closed-loop ledger activity and supporting evidence. These
examples do not move external funds, prove bank settlement or create custody.

## 1. Choose the correct authentication lane

- **Local browser work:** use the same-origin browser BFF. Its signed session,
  CSRF protection, workload credential and actor assertion stay server-owned.
- **Partner server integration:** obtain a short-lived client-credentials token
  from the approved identity provider and send it only to the private API edge.
- Never copy browser cookies, BFF actor assertions, revealed development
  credentials or secret-manager values into source, screenshots, tickets,
  query strings, browser storage or this documentation.

## 2. Preserve exact values

- Transfer input uses an exact decimal JSON string. For example, `"125.50"`
  means 125.50 INR. Do not create or parse this value with binary floating
  point.
- Minor-unit output also remains a JSON string. For example, `"12550"` means
  125.50 INR. Use an integer or decimal library for calculations and format it
  only at the presentation boundary.
- Versions are strings and must be returned unchanged in optimistic commands.

## 3. Persist replay protection before every mutation

Every mutation family—accounts, transfers, funding, corrections,
reconciliation, credentials, webhooks and local preferences—uses the same
client discipline:

1. Create a new opaque `Idempotency-Key` of 16–255 visible characters for one
   logical intent.
2. Persist the key and the complete normalized request before sending it.
3. If the response is lost or times out after send, retry the identical request
   with the identical key.
4. A same-key, same-intent request is a replay and returns the original outcome
   when one exists.
5. A same-key, changed-intent request is an `idempotency_conflict`. Stop and
   inspect the original request; do not disguise the conflict with blind keys.
6. Delete the persisted retry record only after the outcome is known and your
   own durable workflow has recorded the result.

Webhook event deduplication is separate: deduplicate deliveries using
`X-LedgerSync-Event-ID`. Never treat a webhook receipt as financial settlement.

## 4. Canonical transfer in four toolchains

All snippets below use the `createTransfer` operation and the canonical
`CreateTransferRequest` example. A timeout preserves the original key and body.

### curl

```bash
curl --fail-with-body --request POST "$LEDGERSYNC_API_URL/transfers" \
  --header "Authorization: Bearer $LEDGERSYNC_ACCESS_TOKEN" \
  --header "Content-Type: application/json" \
  --header "Idempotency-Key: example-transfer-key-0001" \
  --data '{
    "source_account_id":"70000000-0000-4000-8000-000000000001",
    "destination_account_id":"70000000-0000-4000-8000-000000000002",
    "amount":"125.50",
    "currency":"INR"
  }'
```

### TypeScript

```ts
const idempotencyKey = "example-transfer-key-0001";
const payload = {
  source_account_id: "70000000-0000-4000-8000-000000000001",
  destination_account_id: "70000000-0000-4000-8000-000000000002",
  amount: "125.50",
  currency: "INR",
};

const response = await fetch(`${baseUrl}/transfers`, {
  method: "POST",
  headers: {
    Authorization: `Bearer ${accessToken}`,
    "Content-Type": "application/json",
    "Idempotency-Key": idempotencyKey,
  },
  body: JSON.stringify(payload),
});
// Persist key + payload before send. Reuse both after an unknown response.
```

### Go

```go
body := []byte(`{"source_account_id":"70000000-0000-4000-8000-000000000001","destination_account_id":"70000000-0000-4000-8000-000000000002","amount":"125.50","currency":"INR"}`)
request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/transfers", bytes.NewReader(body))
if err != nil { return err }
request.Header.Set("Authorization", "Bearer "+accessToken)
request.Header.Set("Content-Type", "application/json")
request.Header.Set("Idempotency-Key", "example-transfer-key-0001")
response, err := client.Do(request)
// A timeout after send requires the identical body and idempotency key.
```

### Postman

Import `contracts/generated/ledgersync.postman_collection.json`. Set `baseUrl`,
`bearerToken` and `actorAssertion` only in a private local environment. The
generated collection is an operation catalogue with placeholders, not a
credential store. Never commit a populated Postman environment.

## 5. End-to-end partner lifecycle

1. Authenticate using the partner-server lane.
2. Provision an exact-zero account with `POST /accounts`; retain the returned
   account identifier and request reference.
3. Request controlled funding evidence with `POST /funding-requests`, complete
   the required approval flow and post only the reviewed funding event.
4. Create the transfer using exact decimal text and persisted replay data.
5. Inspect the transfer, account transaction history and related event evidence.
6. Inspect or run reconciliation; never replace unavailable evidence with an
   empty or successful interpretation.
7. Register a webhook endpoint, complete server-initiated ownership
   verification, validate every delivery signature and deduplicate event IDs.

There is no public bulk-provisioning API in contract `2.0.0`. Partner code must
use bounded single-account commands and its own concurrency/back-pressure
limits. The internal host provisioning tool is an operator control, not a
partner endpoint. Do not invent a `/bulk` route or infer bulk support from it.

## 6. Use-case compositions

These patterns reuse the same contracted account, funding, transfer,
reconciliation and webhook primitives. Labels are accounting intent; they do
not change the financial or authorization guarantees.

### Wallet-like balance visibility

- Create one authorized customer-funds account per approved ownership model.
- Record reviewed funding evidence before displaying spendable value.
- Read `available_minor` for availability and ledger transactions for proof.
- Do not call the account a bank wallet or imply custody/external settlement.

### Credit ledger

- Use a reviewed operating/reserve account structure and tenant transfer
  policy; LedgerSync does not decide underwriting or legal credit terms.
- Record drawdown and repayment as explicit, authorized ledger operations.
- Reconcile the complete posting evidence and preserve policy/version context.

### Escrow-like accounting

- Use separately authorized customer-funds and reserve accounts.
- Keep release approval outside ordinary transfer authority and record the
  approved transfer only after the external business condition is satisfied.
- Describe this as escrow-like accounting unless legal owners approve stronger
  custody language.

### Payout accounting

- Record the internal payable and transfer intent in LedgerSync.
- Treat provider payout execution and provider settlement as separate external
  systems until a reviewed provider integration exists.
- Correlate provider references without marking a LedgerSync webhook as payout
  settlement.

### Internal treasury

- Use operating, reserve, payroll, payables and expense categories to express
  purpose; category alone grants no transfer authority.
- Apply role/scopes and tenant transfer policy, then reconcile all postings.
- Use event delivery for notification and the ledger/reconciliation records for
  financial truth.

## 7. Server-initiated webhook verification

1. Create a signing key in the approved secret manager.
2. Register only its non-secret reference and key ID with
   `POST /developer/webhooks`. The endpoint starts as `pending_verification`;
   the API does not return a challenge.
3. LedgerSync posts an expiring challenge directly to the HTTPS endpoint using
   a signed request.
4. Return HTTP 2xx and `X-LedgerSync-Verification: v1=<hex HMAC-SHA-256>` using
   the registered signing key.
5. Poll the endpoint record until it is `active`. A bad proof, expired
   challenge or exhausted retry is not activation.
6. Verify delivery timestamp, key ID and HMAC before use. Acknowledge quickly,
   process asynchronously and use delivery history plus the separately
   authorized approval/replay flow for recovery.

## 8. Unknown, replay and conflict outcomes

- `request_in_progress`, `response_unknown`, `funding_outcome_unknown`,
  `temporary_unavailable` and `transaction_conflict_retryable` preserve the
  original key and intent.
- `idempotency_conflict` means the key is bound to another intent. Stop.
- `reconciliation_already_running` returns the active run reference; inspect it
  instead of queueing a duplicate.
- Rate limiting returns `429` with bounded retry guidance. Preserve mutation
  identity across the wait.

## 9. Safe support evidence

Capture the response `X-Request-ID` and the authorized resource identifier:
`account_id`, `transfer_id`, `funding_event_id`, `run_id`, `event_id` or
`correlation_id`. Share those references, the API version/mode headers and the
non-secret error code. Do not share access tokens, cookies, signing material,
actor assertions, full financial payloads or raw logs.

## 10. Contract and generated artifacts

- Authoritative contract: `contracts/openapi.yaml`.
- Canonical safe UI examples: `contracts/developer-examples.v1.json`.
- Generated TypeScript, Go, manifest and Postman catalogues:
  `contracts/generated/`.
- Reproduce artifacts with `npm --prefix web run generate:developer-artifacts`.
- Prove convergence with `npm --prefix web run check:developer-artifacts` and
  `go test ./tests/contract`.

Generated files are deliberately catalogues rather than published SDK packages.
Do not hand-edit them.
