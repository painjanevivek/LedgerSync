# Outbox backlog or worker loss

1. Verify PostgreSQL remains healthy and inspect the oldest unpublished event, claim lease, attempts, and `last_error_code`.
2. If a worker died, start a single replacement worker. Expired leases are reclaimed; never manually mark events published.
3. If Redis is down, allow the worker to reschedule. The committed transfer, postings, and projection must remain unchanged.
4. Do not describe outbox publication as downstream customer delivery. A posted transfer has `not_applicable` delivery status until an actual delivery adapter appends a durable `delivery_attempts` record.
5. Escalate dead events or age over five minutes. Resolve the dependency cause, then let the worker redeliver idempotently.
6. For a downstream delivery incident, inspect the immutable attempt sequence (`pending`, `retrying`, `delivered`, or `dead`) independently from the financial result. Never change a posted transfer because notification delivery failed.
7. Confirm cache projection recovers and rerun reconciliation before closing a customer-impacting incident.
