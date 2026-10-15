# Unit 3 and Unit 4 evidence

## Source boundary

- Baseline commit: `ed1662e`.
- The source has earlier working changes. Preserve them.
- Use a separate acceptance project and data volumes.
- The study uses ports 3102 and 3103. Product services keep port 3000.
- The product acceptance gate requires a clean committed tree. Keep that gate.
- Study results use a source fingerprint. They do not establish product release approval.

## Step 1: Verified

- Frontend unit baseline: 211 passed; zero failed.
- Go baseline: all selected packages passed with normal temporary-file access.
- Initial sandbox check failed on a temporary health file rename. The repeat check passed without a code change.
- Initial study transfer failed with `validation_failed`. The acceptance helper omitted the current request-reference header. Update the helper and repeat the check.
- The study web port differs from its container port. Use the existing peer readiness check and verify the full HTTP path through smoke tests.
- Docker: 32 CPUs; 12,337,676,288 bytes of memory; engine 29.8.2.
- Financial baseline and same-key replay passed. Internal reconciliation matched with zero mismatches.
- All five real-stack journeys passed. These cover lifecycle, funding, recovery, exports, and investigation handoff.
- Browser baseline: 50 of 51 passed initially. The correction test needed to open its collapsed section. All 38 checks in the follow-up set passed, including that check.
- Four database integration checks passed: cross-instance assertion replay, shared rate limit, concurrent idempotency, and additive correction.
- The first integration connection attempt failed because the private network did not publish test ports. Separate test dependencies now use their own network and loopback ports. Product databases have no host ports.
- [Baseline source and images](baseline.json) and [browser results](baseline-browser.txt) identify the tested source.

## Step 2: Verified

- [Two unique ready APIs](replica-readiness.json) served the isolated environment.
- Session validation, transfer replay, and matched reconciliation passed.
- Shared PostgreSQL controls passed their integration checks in Step 1.
- Configuration, database, existing unit, API, and worker checks passed after the change.
- [Connection budget and identity rules](replica-configuration.md).

## Step 3: Verified

- A private Go gateway checks readiness and refreshes Docker DNS once each second.
- It keeps actor assertions, request IDs, and correlation IDs.
- It streams responses and does not retry commands automatically.
- The Windows race check could not start because CGO is disabled. No race result is claimed.
- Gateway unit checks passed with permission to use local test sockets. The sandbox attempt denied those sockets.
- The first API fault runner read the wrong test-helper status property. The bounded workload stopped and the API was restored. Correct the runner and repeat the drill.
- [API failure evidence](api-failure.json): both APIs served requests; all 40 financial commands resolved; journals remained balanced; reconciliation matched.
- Two real-stack checks passed through the gateway: live investigation handoff and clean-device transfer recovery.
- The failure detection measurement includes the Docker stop command and probe overhead. It is not a network outage measurement.

## Step 4: Verified

- Add failure checks before changing claim behaviour.
- Fence outbox, webhook dispatch, and webhook verification claims.
- Use PostgreSQL time and bounded renewal for each active task.
- The expiry test failed before the fix and passed after it.
- Previous-generation completion, retry, failure, and renewal updates are rejected, including when the owner name is reused.
- Parallel claim and long-task renewal tests passed.
- Real worker processes were killed before publication and after publication. Replacement processes recovered their claims. The second case produced two safe deliveries of the same event.
- Redis pending messages were recovered by a replacement consumer.
- [Two deployed workers](parallel-workers.json) published events. The queue drained without dead events. Reconciliation matched.
- The first worker report had a multi-line JSON formatting error. The report query now returns one JSONB record. Both workloads retained their original financial records.
- Stop old study applications before a claim-contract migration. This is not a mixed-version rolling upgrade.

## Step 5: Verified

- Persist clocks for API and worker identities.
- Keep original event envelopes unchanged during delivery retries.
- Record publication and receive clocks as separate evidence.
- Clock tests passed for concurrency, retry, restart, rollback, overflow, and older envelopes. See [clock rules](clock-rules.md) and [live trace](causal-trace.json).
- The first fixture reused an event type and persistent clock identity. Correct both fixture boundaries. Retain the failed logs.
- The web reader first rejected the added fields. Add strict bounded validation. Preserve exact decimal counters.

## Step 6: Verified

- [Concurrent commands](concurrent-transfers.json) did not overspend or create duplicate journals. Reconciliation matched.
- Retain the first path, lock-conflict, and query failures. Correct the driver. Use three bounded identical retries for a retryable conflict.
- Provision test-only database roles before integration checks. The corrected suite passed.

## Step 7: Verified

- [Snapshot evidence](snapshot-report.json) records one repeatable-read ledger boundary. Redis and outbox observations have separate sample times.
- Concurrent reconciliation, cache loss, stale cache, and database unavailability checks passed.
- The first runner used the wrong balance path and assumed a timestamp was a string. Correct the runner and retain its failed logs.

## Step 8: Verified

- [Scheduler evidence](scheduler-takeover.json) records replacement after stop and pause. Stale claims were rejected. Scheduled intervals remained unique. Manual reconciliation matched.
- Lease failure cancels work. The restricted identity cannot read base financial tables or write transfers.
- The first scheduler test ran before its migration. The later identity fixture needed session authorization. Retain both failures and corrected results.

## Step 9: Verified

- [Browser checks](ui-review-fix-verified.txt) passed for desktop, mobile, keyboard access, actual observations, event order, snapshot details, and evidence download.
- The accessibility scan found no violations. Root layout did not overflow at 1440px and 390px.
- Frontend tests: 214 passed. Lint and type checks passed. Go route and repository checks passed.
- Impeccable context and detector launchers failed because the engine was absent and its cache was not writable. Read project context directly.
- The fresh reviewer found a retained ACTIVE lease badge. Use sampled-state wording and an expiry timer. The reviewer scored that correction resolved.
- See [design documentation](ui-design-review.md). This extends the current visual system.

## Step 10: Verified

- Eight 30-second workloads passed with a five-second warm-up, ten paced clients, and fresh seed data before each run. All k6 thresholds passed.
- Each measured run posted 300 new transfers. No unresolved error, business rejection, duplicate journal, or unbalanced journal occurred. Delivery drained. Reconciliation matched.
- [Measured results](performance-report.md) retain latency increases and decreases. Throughput stayed near 10.2 commands per second under the fixed demand.
- Per-instance resource limits stay fixed. Total limits increase with replicas. This short test does not establish maximum production capacity.
- Repeat API loss, worker process crashes before and after publication, and scheduler stop/pause. Each recovery drill reconciled. See [recovery timings](recovery-report.json).
- The separate benchmark project and volumes were removed. The original study environment was restored.

## Step 11: Verified

- All 42 Go packages passed. The log records 423 top-level passing tests. Go static checks passed.
- All 214 frontend unit checks passed. Lint, types, and the production build passed.
- All 245 browser checks passed with snapshot updates disabled. All six real-stack financial and evidence journeys passed.
- Final isolated acceptance verified a normal financial command, identical replay, and matched reconciliation.
- Fix the export scanner query, missing replica count, contract catalogue, and outdated lease and migration assumptions. Keep the failed logs.
- Repair browser timing and old visual fixtures. Preserve financial permissions and mixed-currency refusal. Verify the current console again.
- The normal workspace is healthy at port 3000. Its six accounts, four transfers, ten postings, and financial fingerprints match the values before the upgrade.
- All 14 normal pages rendered. The normal configuration does not infer study replica counts or start the scheduler.
- The first cleanup left profiled services. Enable all profiles for the exact study identity. No study containers, volumes, or networks remain. Remove only the task image tags.
- Docker Desktop stopped after the normal upgrade. Restart it and repeat the normal health and data-preservation checks.
- See [final validation](final-validation.md), [handover instructions](handover.md), and [commit messages](commit-messages.md).
- The Windows symlink check lacked creation privilege. The parent crash helper intentionally skipped. No Windows race or production capacity claim is made.
