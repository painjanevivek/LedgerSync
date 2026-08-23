# Phase 7 — reviewed retry-safe transfer evidence

## Implemented journey

1. Prepare selects only active, authorized, same-currency accounts and excludes the source from destination choices.
2. Exact decimal text is converted to integer minor units without floating-point arithmetic.
3. Review keeps source, destination, amount, and currency together and preserves edits when returning.
4. Confirm creates the idempotency key immediately before the first submission and disables repeat confirmation while in flight.
5. A lost/unknown response retains the same key in session storage and exposes only `Retry same transfer`.
6. Success removes the key, preserves the confirmed transfer ID, links both accounts and the permanent transfer record, and refreshes affected evidence.
7. Insufficient funds and validation failures explicitly state that no money moved. Idempotency conflict requires a genuinely new intent.

## Automated proof

- malformed precision is rejected before an API call;
- lost first response followed by retry sends the same non-empty idempotency key twice;
- insufficient funds states that no movement occurred;
- the transfer detail separates `financial_status` from `delivery_status`;
- compact and desktop routes preserve exact amount, record ID, posting evidence, and account links.

The existing backend suites continue to cover sequential/concurrent duplicates, same-key/different-payload conflict, transactional rollback, cross-tenant authorization, cache loss, outbox retry, and Redis fault behavior.
