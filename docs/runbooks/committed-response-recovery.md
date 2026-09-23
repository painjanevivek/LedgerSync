# Committed response recovery

Use this runbook when a transfer, funding or correction command completed
durably but optional metadata or the HTTP response body could not be delivered.

## Meaning

- A 2xx status or a `committed` recovery envelope is a final durable outcome,
  even when `metadata_status` is `partial` or `unavailable`.
- A client disconnect after the command left the BFF is an unknown client-visible
  outcome, not a database failure. Never submit a different command to compensate.
- Consistency requirements are optional read acceleration metadata. Their absence
  cannot reverse or invalidate a committed financial result.

## Safe recovery

1. Capture the correlation ID, command kind, idempotency key and bounded command
   ID hash. Do not copy amounts, account IDs, tokens or response bodies into alert
   channels.
2. For a known command ID, read the canonical resource with its documented GET
   endpoint. PostgreSQL remains authoritative.
3. When the command ID is unknown, repeat the identical command payload with the
   identical idempotency key. A replay must return the same final financial status.
4. Treat missing or malformed consistency headers as unavailable metadata. Drop
   them, keep the 2xx result and perform the next read without a requirement token.
5. Page the owning team if write failures repeat or metadata failures exceed the
   deployment baseline. Do not rewrite a committed success as a synthetic 5xx.

## Verification

- The GET or idempotent replay agrees with the original durable result.
- No duplicate journal or posting was created.
- `committed_response_write_failures` and
  `committed_response_metadata_unavailable` return to baseline.
- Logs contain command kind and bounded ID hash only; no token, amount, account or
  response body is present.
