# Copyable commit messages

These messages describe this plan's changes. They do not include earlier working changes.
Do not mark a phase accepted until its checks pass.

## Step 1: Study setup and baseline test repair

```text
feat(study) : add an isolated Unit 3 and Unit 4 test environment

- Reuse local acceptance helpers with separate project state and data volumes.
- Keep the normal workspace and its release gates intact.
- Record source fingerprints, image identifiers, resources, and baseline results.
- Use ports 3102 and 3103 for the study browser sessions.
```

```text
fix(acceptance) : align baseline checks with the current transfer contract

- Keep a stable request reference when a transfer command is retried.
- Open correction details before testing the step-up return path.
- Retain failed checks in the study evidence record.
```

## Step 2: Replica configuration

```text
feat(runtime) : prepare API and worker replicas for the course study

- Give each service instance a unique identity that survives process restart.
- Bound each database pool and record the total connection budget.
- Share protected signing configuration and persistent command controls.
- Verify both API instances, authenticated transfer replay, and reconciliation.
```

## Step 3: Private API gateway

```text
feat(study) : balance private API requests between ready instances

- Refresh Docker DNS addresses and check readiness before routing.
- Preserve signed actor headers, request identifiers, and streaming responses.
- Return a controlled unavailable result without automatic command retries.
- Verify live investigation, transfer recovery, and a 40-command failure drill.
```

## Step 4: Worker fencing and recovery

```text
fix(workers) : reject expired and replaced claims with generation fences

- Use PostgreSQL time for outbox and webhook claim expiry.
- Require the current generation for completion, retry, failure, and renewal.
- Renew active tasks and cancel dispatch when ownership is lost.
- Store publication attempts separately from immutable event envelopes.
- Verify two workers, process crashes, Redis pending recovery, and balanced reconciliation.
```

## Step 5: Causal event metadata

```text
feat(events) : add durable Lamport clocks and causal delivery evidence

- Stamp new events in the financial outbox transaction.
- Apply send and receive rules across API and worker instances.
- Preserve origin metadata during retries and record attempts separately.
- Accept older events and validate exact counters in the web reader.
- Verify restart, concurrency, overflow, delayed messages, and reconciliation.
```

## Step 6: Mutual exclusion

```text
feat(study) : demonstrate financial mutual exclusion across API replicas

- Run competing debits and opposite-direction transfers.
- Verify shared idempotency and concurrent reconciliation.
- Retain controlled conflicts and use bounded identical retries.
- Check final balances, journal totals, and reconciliation.
```

## Step 7: Snapshot evidence

```text
feat(reconciliation) : record delivery counts in the ledger snapshot

- Preserve the repeatable-read boundary and account scope.
- Store pending, published, and dead event counts in that snapshot.
- Report separate collection times for current service observations.
- Verify concurrent transfers, stale cache protection, and dependency recovery.
- Preserve unavailable metadata for older reconciliation records.
```

## Step 8: Scheduler takeover

```text
feat(reconciliation) : add optional scheduler leases and safe takeover

- Keep scheduled operation disabled by default.
- Fence scheduled jobs with database time and lease generations.
- Restrict the scheduler identity to configured tenants and reporting actions.
- Recover work after stop and pause without duplicate scheduled intervals.
- Verify stale-owner rejection and manual reconciliation.
```

## Step 9: Technical evidence UI

```text
feat(ui) : add distributed system evidence behind existing disclosures

- Load service observations when the Developer section opens.
- Show event clocks and reconciliation snapshot boundaries.
- Label historical, unavailable, and expired lease evidence.
- Preserve the current navigation and shared design tokens.
- Verify keyboard access, mobile layout, and authorization.
```

## Step 10: Scaling and recovery

```text
feat(study) : measure replica scaling and controlled recovery

- Compare all four API and worker configurations with fresh synthetic data.
- Run independent-account and shared-account workloads through normal controls.
- Record command latency, throughput, queue age, errors, and resource samples.
- Reconcile every workload and repeat the approved failure drills.
- Report improvements, unchanged throughput, regressions, and measurement limits.
```

## Step 11: Regression repairs

```text
fix(evidence) : preserve exports and strict observations after the study changes

- Include snapshot details in reconciliation export queries.
- Reject missing, stale, and oversized replica observations.
- Keep the Developer endpoint catalogue aligned with the API contract.
- Update migration and recovery checks for database lease time.
- Verify manual reconciliation by its saved record ID.
```

## Step 11: Browser fixture repairs

```text
fix(ui-tests) : align browser fixtures with the current console

- Use a fixed clock for repeatable visual captures.
- Open existing disclosures before checking their controls.
- Use current headings and preserve permission checks.
- Wait for request dispatch and the sign-out response.
- Verify the refreshed baselines without update mode.
```

## Step 11: Final validation and handover

```text
feat(study) : complete Unit 3 and Unit 4 validation and handover

- Verify Go checks, financial journeys, and the full browser suite.
- Record source, image, performance, and recovery evidence.
- Confirm normal service health and preserved financial records.
- Document scheduler shutdown and forward migration recovery.
- Remove the isolated test resources and complete the phase register.
```
