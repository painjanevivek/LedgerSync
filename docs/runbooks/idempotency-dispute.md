# Idempotency dispute

1. Obtain tenant, actor, operation, original idempotency key, correlation ID, and customer-provided request timestamp. Never place payment tokens or raw account balances in the ticket.
2. Look up the stored idempotency request and transfer outcome in PostgreSQL primary.
3. A matching request fingerprint must replay its saved response; a different fingerprint with the same key must remain a conflict.
4. Use transfer ID, journal transaction, postings, and audit record to explain the result. Do not issue a second transfer to "test" the dispute.
5. If correction is justified, create a reviewed compensating transfer/journal and preserve the original immutable evidence.
