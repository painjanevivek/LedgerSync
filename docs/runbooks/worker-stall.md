# Worker stall response

Use this runbook when a worker heartbeat is stale, an active queue stops making
progress, or the webhook signing-key cache repeatedly reaches capacity.

## Safety rules

- Do not replay, unlock, delete or directly update financial/outbox/webhook rows.
- Treat PostgreSQL as authoritative and preserve the current job/attempt evidence.
- Do not copy signing-key references, key material, tenant IDs or endpoint URLs
  into alerts, tickets, chat or metric labels.
- Prefer a controlled worker restart only after capturing the bounded evidence
  below. At-least-once delivery means a restart can produce a safe duplicate.

## Triage

1. Confirm whether `ledgersync_worker_heartbeat_unix_seconds` is stale for all
   queues. All queues stale indicates a stopped process or telemetry path; one
   active queue with a rising progress age indicates blocked work.
2. Record the queue, deployment/image version, first alert time, last-started and
   last-completed timestamps, and the hashed item identifier from controlled
   worker diagnostics. Never record the raw item identifier.
3. For `webhook_delivery`, compare key-cache lock-wait, resolution outcomes and
   capacity evictions. A fresh heartbeat with a rising active age distinguishes a
   stuck delivery from a dead process.
4. Inspect dependency health and bounded logs. Do not increase cache capacity to
   mask repeated churn; the cache remains limited to 128 live entries by design.
5. If progress remains stalled beyond the alert horizon, drain and restart the
   affected worker version. Confirm the lease horizon has elapsed before any
   manual recovery action.

## Recovery verification

- Heartbeat age returns below 30 seconds.
- The affected queue completes work and active progress age resets.
- Capacity evictions and lock wait return to baseline.
- Outbox/webhook attempts remain auditable and no raw signing material appears in
  metrics or logs.
- If restart does not restore progress, escalate with the bounded evidence and
  keep manual database mutation disabled.
