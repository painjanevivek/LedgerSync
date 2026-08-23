# Outbox backlog or worker loss

1. Verify PostgreSQL remains healthy and inspect the oldest unpublished event, claim lease, attempts, and `last_error_code`.
2. If a worker died, start a single replacement worker. Expired leases are reclaimed; never manually mark events published.
3. If Redis is down, allow the worker to reschedule. The committed transfer, postings, and projection must remain unchanged.
4. Do not describe outbox publication as downstream customer delivery. A posted transfer has `not_applicable` delivery status until an actual delivery adapter appends a durable `delivery_attempts` record.
5. Escalate dead events or age over five minutes. Resolve the dependency cause before replay. Never update `dead_at`, `published_at`, attempt numbers, or financial rows by hand.
6. For a downstream delivery incident, inspect the immutable attempt sequence (`pending`, `retrying`, `delivered`, or `dead`) independently from the financial result. Never change a posted transfer because notification delivery failed.
7. Confirm cache projection recovers and rerun reconciliation before closing a customer-impacting incident.

## Reviewed outbox replay

Use two different authorized operators and one approved change/ticket UUID:

```text
go run ./cmd/replay-outbox -action inspect -tenant-id <tenant-uuid> -event-id <event-uuid>
go run ./cmd/replay-outbox -action approve -tenant-id <tenant-uuid> -event-id <event-uuid> -actor-subject-id <approver> -reason-code dependency_restored -correlation-id <change-uuid>
go run ./cmd/replay-outbox -action replay -tenant-id <tenant-uuid> -event-id <event-uuid> -actor-subject-id <executor> -correlation-id <change-uuid>
```

The replay keeps the original event ID and financial payload, clears only the dead delivery lease/retry state, and appends approval, execution, and audit evidence atomically. Repeating execution cannot create a second execution record.

## Reviewed downstream-delivery retry

```text
go run ./cmd/replay-delivery -action inspect -tenant-id <tenant-uuid> -attempt-id <dead-attempt-uuid>
go run ./cmd/replay-delivery -action approve -tenant-id <tenant-uuid> -attempt-id <dead-attempt-uuid> -actor-subject-id <approver> -reason-code endpoint_restored -correlation-id <change-uuid>
go run ./cmd/replay-delivery -action replay -tenant-id <tenant-uuid> -attempt-id <dead-attempt-uuid> -actor-subject-id <executor> -correlation-id <change-uuid>
```

This appends a new pending attempt with the next attempt number. It never rewrites the dead attempt or the posted transfer. The downstream recipient must deduplicate on the stable transfer/event identity; pause replay if that recipient contract is not proven.

## Redis stream bound

`LEDGERSYNC_REDIS_STREAM_MAX_LENGTH` defaults to 5,000,000 and is applied approximately by Redis. At 80% depth or sustained consumer lag, scale or repair projectors and verify PostgreSQL outbox replay coverage. Do not increase the bound until Redis memory and recovery-time impact are measured. Redis may be discarded and rebuilt; PostgreSQL remains authoritative.
