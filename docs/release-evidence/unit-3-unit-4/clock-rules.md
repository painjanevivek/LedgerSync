# Logical event order

## Persistent participants

API and worker instances have separate node identities. Each identity has one durable counter in PostgreSQL. Concurrent updates lock that counter row.

1. Before an API records an outbox event, increase its local counter by one.
2. Store that counter with the outbox event in the financial transaction.
3. When a worker receives that event, set its counter to `max(local, origin) + 1`.
4. Before publication, increase the worker counter by one again.
5. Store both worker values in the current publication attempt.
6. Send the immutable origin envelope and the separate publication metadata through Redis.
7. When a projection worker receives the message, set its counter to `max(local, publication) + 1`.
8. Record that receipt before the cache consumer processes the message.

## Retry and restart

- A retry keeps the event identifier, origin node, origin counter, and parent reference.
- A new claim records a separate publication attempt and new worker counters.
- Redis transport retries of one prepared message keep its prepared metadata.
- A process restart uses the stored counter for its existing node identity.
- A replacement container uses a new identity unless an operator retains its unique name.
- A transaction rollback also rolls back its clock update.
- Overflow stops the operation. A counter never wraps to zero.
- Older events have no origin metadata. Their origin is unavailable. A receiver can still record its local receipt.

## Limits

A transfer parent reference identifies the financial record that produced the event. Receipt records link message send and receive evidence. A larger counter alone does not prove causation between unrelated events.

Aggregate versions continue to order balance cache updates. PostgreSQL continues to decide postings. This is not a physical clock or a consensus protocol.
