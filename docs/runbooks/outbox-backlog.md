# Outbox backlog or worker loss

1. Verify PostgreSQL remains healthy and inspect the oldest unpublished event, claim lease, attempts, and `last_error_code`.
2. If a worker died, start a single replacement worker. Expired leases are reclaimed; never manually mark events published.
3. If Redis is down, allow the worker to reschedule. The committed transfer, postings, and projection must remain unchanged.
4. Escalate dead events or age over five minutes. Resolve the dependency cause, then let the worker redeliver idempotently.
5. Confirm cache projection recovers and rerun reconciliation before closing a customer-impacting incident.
