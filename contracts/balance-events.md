# Balance event compatibility

The durable outbox envelope identifies the tenant, event, account aggregate and exact aggregate version. Workload processing remains tenant scoped and lease fenced. Consumers must deduplicate event identifiers and apply only a later exact integer version. Do not compare versions through a floating-point number.

`account.balance.changed.v1` carries `available_minor`, currency, version, account/event identity and occurrence time. Historical version 1 implies posted ledger equals available and reserved equals zero. Retain that interpretation for old events.

`account.balance.changed.v2` carries the same operational fields plus exact `ledger_minor` and `reserved_minor` strings. Required equation: `ledger_minor - reserved_minor = available_minor`. Each value is a nonnegative customer int64 minor-unit value; reserved cannot exceed posted ledger. The payload identity and version must agree with the durable envelope. A new producer must use version 2 whenever the old equality assumption is false. Reserve, release and expiry use version 2. A capture emits only the existing transfer's final balance change; a temporary internal release is not an event.

An account balance event has exactly one durable source: transfer, funding event, opening import, or hold. Its outbox foreign key retains that source even after a hold becomes terminal. Hold events are not bank settlement notifications. Redis is a disposable monotonic projection. An API balance read still obtains PostgreSQL truth before a cache answer can be exposed.

A consumer that supports only version 1 is not a compatible rollback when version-2 events or active holds exist. Retain a compatible consumer and expiry worker. Do not discard version-2 events or lower stored versions to make old code accept them. See [reservation semantics](../docs/portfolio/reservations.md).
