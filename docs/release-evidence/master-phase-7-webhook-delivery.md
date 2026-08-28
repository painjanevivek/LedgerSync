# Master plan Phase 7 — webhook delivery evidence

## Scope

- A posted transfer transaction emits one immutable `transfer.posted` event and schedules one durable job for every active matching endpoint.
- The worker dispatches a bounded HTTPS POST signed with HMAC-SHA-256. It has no proxy path, follows no redirects, rejects private destination addresses at dial time, and never records raw signing keys.
- Each observed outcome appends a `delivery_attempts` evidence row. Retryable transport/408/429/5xx outcomes use bounded exponential retry; non-retryable or exhausted outcomes dead-letter.
- Replay requires prior approval by another actor and creates an approval-linked durable job. It cannot rewrite the original dead attempt or event payload.

## Local validation

Executed on the implementation commit before push:

```text
node scripts/generate-developer-artifacts.mjs
go test ./... -count=1
git diff --check
```

The local test invocation exercised unit, contract, fault, and available integration suites. Database-backed migration and delivery lifecycle tests run in CI when `LEDGERSYNC_TEST_DATABASE_URL` is provisioned.
