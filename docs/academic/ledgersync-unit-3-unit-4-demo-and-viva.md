# LedgerSync: Unit 3 and Unit 4 demonstration and viva

Prepared: 12 October 2026. Based on the local implementation and saved study evidence.

UNIT 3 + UNIT 4  |  Prepared 12 October 2026

## LedgerSync
Demo and viva

### What we built

LedgerSync is an internal financial ledger with a web console and background workers. It records exact amounts, balanced journals, and transfer evidence. The course additions show how services coordinate, process events, and recover from selected failures.

Use this opening: 'Our project is a distributed web application for an internal ledger. PostgreSQL stores the financial record. Docker runs separate API and worker services. We demonstrate logical clocks, database coordination, consistent reports, and controlled failover.'

### Your 10-minute presentation

| Time | Show | Explain |
| --- | --- | --- |
| 0-1 min | Architecture on page 2 | Separate services; one financial authority. |
| 1-3 min | Developer observations | Two ready APIs; recorded worker activity; sampled scheduler lease. |
| 3-5 min | Event order | Follow one event through origin, publication, and receive clocks. |
| 5-6 min | Snapshot boundary | One consistent PostgreSQL report. |
| 6-8 min | Saved concurrency and recovery evidence | No overspend; safe replay; replacement after selected failures. |
| 8-10 min | Measured results and limits | Fixed demand, one host, no production capacity claim. |

### Prepare before the class

Build and rehearse the study stack before the presentation. Keep the PDF and evidence files offline. Run the longer checks before class. During the presentation, show the UI and saved reports. Run a live fault check only if time and Docker are available.

The guide describes existing features. Creating this PDF did not run financial commands or fault checks.

Use the syllabus terms accurately. A related mechanism is not the same as every named algorithm.

## Map the additions

Architecture: Web/BFF -> private gateway -> two Go APIs -> PostgreSQL ledger/outbox.
PostgreSQL outbox -> two workers -> Redis -> projection consumers.
Two optional schedulers -> PostgreSQL lease coordination -> internal reports.

| Syllabus area | Implemented addition | Purpose |
| --- | --- | --- |
| Unit 3: logical clocks / Lamport | Durable origin, send, and receive counters | Explain causal message order. |
| Unit 3: mutual exclusion | Account locks, advisory locks, fenced claims | Prevent conflicting financial and worker updates. |
| Unit 3: global state | One repeatable-read database snapshot | Report a consistent ledger boundary. |
| Unit 3: election | Database lease with scheduler takeover | Select one current scheduling owner. |
| Unit 4: distributed web systems | Web, private gateway, API and worker replicas | Show communication and independent service scaling. |
| Mini-project deployment | Isolated Docker Compose stack | Run repeatable demonstrations with synthetic data. |

Kubernetes, serverless platforms, Hadoop, distributed file systems, vector clocks, physical clock synchronization, blockchain, and the named election/fair-mutex/deadlock algorithms are not implemented here. Docker satisfies the stated deployment requirement. The snapshot is not a Chandy-Lamport global snapshot; the scheduler is not Bully, Ring, or Raft.

PowerShell commands run from D:\Work\Project\Dev\LedgerSync.

## Start the study stack

### 1. Preserve the saved evidence

Some study scripts replace report files. Before a new run, copy the saved evidence to a new dated folder. Keep failed results as well as successful results.

```powershell
Set-Location 'D:\Work\Project\Dev\LedgerSync'
$backupName = 'unit-evidence-' + (Get-Date -Format 'yyyyMMdd-HHmmss')
New-Item -ItemType Directory -Path '.\tmp\presentation' -Force | Out-Null
Copy-Item -LiteralPath '.\docs\release-evidence\unit-3-unit-4' `
  -Destination (Join-Path '.\tmp\presentation' $backupName) -Recurse
```

### 2. Start Docker Desktop and build the isolated stack

Keep Docker Desktop running. The last saved cleanup removed the study containers and image tags. Rebuild on the first start. Do not add -SkipBuild to this first command.

```powershell
.\scripts\start-unit-study.ps1 `
  -APIReplicas 2 -WorkerReplicas 2 -IncludePeer -WithScheduler
.\scripts\test-unit-study-final.ps1
```

Expected: a financial command passes, the identical replay passes, and reconciliation matches. The final check uses a synthetic amount of 100 minor units. With INR, this is INR 1.00. It changes only study data.

### 3. Open the correct workspace

| Address | Use |
| --- | --- |
| http://127.0.0.1:3000/ | Normal workspace; scheduler disabled by default. |
| http://127.0.0.1:3102/ | Synthetic study; use this for the presentation. |
| http://127.0.0.1:3103/ | Second web instance for cross-instance journeys. |

Use the existing local sign-in flow. Do not show runtime.env, credentials, cookies, or assertions. The study uses a separate Compose identity and data volumes. Its database and Redis remain private.

The new details stay inside existing pages. Open the disclosure when you need technical evidence.

## Show the UI locations

### A. Developer: replicas and scheduler

1. In the study at port 3102, open Platform in the sidebar. Select Developer (/developer). If the sidebar is collapsed, expand it first.

2. Expand Distributed systems. Read Ready API replicas and its collection time. With both APIs ready, expect 2.

3. Read Historical publication evidence. It counts worker identities in the latest 1,000 published attempts. It is not a count of workers running now.

4. Read Active scheduler. Show its owner, generation, lease expiry, and sample time. Select Refresh observations for a new sample.

5. Select Download historical test evidence. Explain that this is a saved report asset, not a live stream of results.

Say: 'These observations have separate collection times. An active lease is active at the sample time. A retained sample becomes historical.'

### B. Events & webhooks: logical event order

1. Open Investigate, then Events & webhooks (/events). Select a recent study event to open its details.

2. Expand Event order. Show the origin node, logical timestamp, and parent reference.

3. Read Publication attempts and Receive clocks. Follow the same event ID. Delivery attempts remain a separate section.

### C. Internal reconciliation: snapshot boundary

1. Open Internal reconciliation (/reconciliation). Select a recent completed study report.

2. Read the result and mismatch count. Expand Snapshot boundary.

3. Show Repeatable read, schema version, ledger watermark, database snapshot time, and recorded delivery counts.

Older events and reports can lack the added metadata. Select a new study record. The UI must show unavailable fields honestly.

Use one event and one completed report. Avoid claims that the evidence cannot support.

## Explain clocks and snapshots

### Demo 1: follow a causal message

If you need a fresh trace, run this before class. It creates study events, waits for receipts, checks order, and saves causal-trace.json.

```powershell
.\scripts\test-unit-study-clocks.ps1
```

1. Open the saved trace. Find trace.origin and choose an event ID. Open that event in the study UI at /events/<event-ID>.

2. Read the API origin counter. The API increases its counter before it records the outbox event.

3. Read the worker receive counter. It must exceed the origin counter. Read the publication send counter; it must exceed that receive counter.

4. Read the consumer receipt. Its receive counter must exceed the publication send counter.

```powershell
Local event:      C = C + 1
Message receive:  C = max(C_local, C_sender) + 1
Example only:     origin 12 -> worker receive 13 -> send 14 -> receive 15
```

Say: 'These are logical counters, not wall-clock times. A message receive follows its send. A larger counter between unrelated events does not prove causation. Aggregate versions still control cache update order.'

### Demo 2: read one consistent report

1. Open a completed reconciliation result. State its tenant scope and ledger watermark.

2. Expand Snapshot boundary. Read its database sample time and the pending, published, and stopped delivery counts.

3. Explain that the financial checks and recorded outbox counts use one repeatable-read PostgreSQL transaction.

Say: 'This report checks internal ledger consistency at one database boundary. Redis and process observations have separate times. A matched report does not prove bank settlement.'

For a longer prepared check, use test-unit-study-snapshot.ps1. It includes cache loss, stale cache, and database unavailability. Use its saved snapshot-report.json during the short presentation.

Use the saved reports for a short presentation. The scripts below run only in the isolated study.

## Show safe concurrency

### Demo 3: competing transfers

```powershell
.\scripts\test-unit-study-concurrency.ps1
```

1. Open concurrent-transfers.json. Show the source funded with 10 units and 20 simultaneous debit attempts of 1 unit.

2. Show the result: 10 transfers posted and 10 were rejected. The account did not overspend.

3. Show the opposite-direction transfer check and the same-key request check across API instances. There is one financial result for the repeated command.

4. Show matched reconciliation and balanced journals. Explain that the controls use PostgreSQL locks and constraints shared by both APIs.

Say: 'Mutual exclusion protects shared account state. Idempotency protects one command from duplicate movement. They solve different problems.'

### Demo 4: parallel workers

```powershell
.\scripts\test-unit-study-workers.ps1
```

1. Open parallel-workers.json. Show two distinct worker identities processing the study workload.

2. Show that pending work drains and no dead events remain in that recorded check.

3. Explain SKIP LOCKED claims: a worker skips rows already locked by another worker. Each committed claim has an owner, expiry, and increasing generation.

4. Explain fencing: the database rejects an acknowledgment from an expired or replaced generation. A lease alone cannot reject an old process after it resumes.

### Keep posting separate from delivery

A transfer can be posted while its downstream event is pending. The financial transaction commits the ledger and outbox together. Workers publish after that commit. Delivery is at least once. A repeated event must keep its event ID, and consumers must handle duplicates.

A worker identity in a historical report proves recorded activity. Use readiness or a fresh process check to discuss current service state.

Do these drills during rehearsal. They can stop or pause study services.

## Show controlled failover

### Demo 5: lose one API

```powershell
.\scripts\test-unit-study-api-failure.ps1
```

1. Before the drill, refresh Developer observations and show two ready APIs.

2. Run the script in the study PowerShell session. It sends a bounded workload, stops one API, and restores it in cleanup.

3. Open api-failure.json. Show that all 40 commands resolved, journals stayed balanced, and reconciliation matched.

4. Explain the interrupted attempt: the caller repeats the same command with the same key. The gateway does not silently retry financial commands.

Recorded detection was 1,661.84 ms. This includes Docker command and probe overhead. It is not a production availability guarantee.

### Demo 6: replace the scheduler owner

```powershell
.\scripts\test-unit-study-scheduler.ps1
```

1. In Developer, show the sampled scheduler owner and generation. The study starts two scheduler candidates.

2. Run the prepared check. It stops and pauses the current owner in separate cases. A replacement obtains the expired lease.

3. Open scheduler-takeover.json. Show the replacement, rejected stale acknowledgment, and unique scheduled intervals.

4. Refresh the UI. Show the new owner. Show that manual reconciliation still works.

Say: 'PostgreSQL coordinates leader selection with database time and a fencing generation. The scheduler can create reports for its allowed synthetic tenant. It cannot submit transfers.'

Study takeover: 4,901 ms after stop; 5,025 ms after pause. Study lease: 4 seconds. Normal defaults differ. A report can repeat after an unknown completion, but the scheduler does not repeat a financial movement.

Read the saved measurements. Do not turn a small experiment into a capacity claim.

## Present measured results

### The recorded comparison

Each configuration used 10 paced clients, a 5-second warm-up, and a nominal 30-second workload. Each run posted 300 new transfers and included 30 replays. Data was reset between runs. Each API and worker had 1 CPU, 256 MiB memory, and 4 database connections. Total resources grew with replicas.

| API / worker | Independent p95 | Contended p95 | Posted/s range |
| --- | --- | --- | --- |
| 1 / 1 | 187.00 ms | 287.55 ms | 10.24-10.29 |
| 2 / 1 | 192.30 ms | 207.20 ms | 10.28-10.28 |
| 1 / 2 | 240.00 ms | 168.10 ms | 10.19-10.29 |
| 2 / 2 | 194.55 ms | 214.10 ms | 10.25-10.28 |

All eight runs recorded no unresolved commands, duplicate journals, or unbalanced journals. Delivery drained and internal reconciliation matched. Latency improved in some runs and increased in others. Throughput stayed near the fixed offered demand.

### What you can conclude

The system supports separate API and worker replicas. Shared controls kept the financial results correct in these experiments. The tested failures recovered. The fixed-demand test did not prove that more replicas increase maximum throughput.

### What you must qualify

These were short tests on one Docker host. PostgreSQL was a common dependency. The account and tenant policy locks limit concurrent commands. The tests do not establish multi-host availability, maximum capacity, sustained production load, or statistical significance.

Worker crash recovery was 620 ms before publication and 615 ms after publication. Both include a deliberate 600 ms wait. The second case delivered the same event twice. Do not present these as intrinsic production recovery times.

### Evidence to open

Use performance-report.md, recovery-report.json, api-failure.json, scheduler-takeover.json, and worker-crash-verified.txt under docs/release-evidence/unit-3-unit-4/.

The full performance runner can take time and temporarily stops the original study database and cache. Use its saved report in class. It leaves the normal workspace outside its boundary.

Keep a fallback ready. A missing observation must stay missing; do not invent a value.

## Finish and recover safely

| Problem | Action |
| --- | --- |
| API count unavailable at port 3000 | Open the study at 3102. The normal workspace does not infer study counts. |
| Old event has no clocks | Select the new event ID from causal-trace.json. |
| Old report has no snapshot counts | Select a new completed study report. |
| Lease is historical | Refresh observations. Check that the scheduler profile is running. |
| Access denied | Use an authorized local session. Developer observations require developer:read. |
| Docker or build fails in class | Show this guide and the saved evidence. State that the evidence is recorded, not live. |

### Stop the optional study scheduler

```powershell
. .\scripts\unit-study-common.ps1
Invoke-LedgerSyncCompose -ComposeArguments @(
  '--profile','scheduler','stop','reconciliation-scheduler'
)
```

### Remove only the isolated study when you finish

This removes synthetic study data. Check the exact project name before the remove command. Do not use a global prune or target the normal compose project.

```powershell
. .\scripts\unit-study-common.ps1
$studyState.project
Invoke-LedgerSyncCompose -ComposeArguments @(
  '--profile','*','down','--volumes','--remove-orphans'
)
```

Use a fresh PowerShell session for normal workspace commands. Keep the saved reports and backup. Normal start/status commands are scripts/start-local.ps1 and scripts/status-local.ps1.

### How to describe the product

'LedgerSync is an internal ledger and operations console. It records and checks financial movements within its configured scope. It also demonstrates distributed service coordination.' It is not a bank, a payment rail, a custodian, or a blockchain network.

Give the short answer first. Add the evidence only when the examiner asks.

## Viva: project basics

### 1. What is LedgerSync?

It is an internal financial ledger with a web console and workers. PostgreSQL stores the authoritative financial record. The study adds distributed coordination evidence.

### 2. What does distributed mean in this project?

Separate web, API, worker, database, and cache processes communicate across Docker networks. They run on one host in this study. This does not prove multi-machine fault tolerance.

### 3. What is a ledger?

A ledger records financial entries. LedgerSync keeps journal postings and account balances. Committed financial evidence remains available for investigation.

### 4. What is double-entry accounting?

Each journal has equal total debits and credits. This helps detect an unbalanced movement. It does not prove that the business reason for the movement was correct.

### 5. How are amounts stored?

Amounts use exact integer minor units. The API preserves exact decimal strings. With INR, 100 minor units equal INR 1.00. Floating-point arithmetic is unsuitable for exact ledger amounts.

### 6. What is idempotency?

The same command, key, and body return the original financial result instead of creating another movement. Reusing the key with a different payload is rejected.

### 7. Why is PostgreSQL authoritative?

It commits financial updates, postings, audit evidence, and the outbox atomically. Redis is disposable. A cache value cannot replace an authoritative financial check.

### 8. What is the outbox pattern?

The financial transaction stores an event in PostgreSQL with the ledger change. A worker publishes that committed event later. This closes the gap between committing a movement and recording its delivery work.

### 9. What is internal reconciliation?

It checks ledger consistency within the recorded scope and snapshot. A matched report is internal evidence. It is not proof that a bank transfer settled.

### 10. What is the practical use case?

A team can record internal movements, review funding evidence, and investigate posting or delivery state. A real deployment also needs suitable operational, legal, and business controls.

Give the short answer first. Add the evidence only when the examiner asks.

## Viva: Unit 3

### 11. What is a Lamport clock?

It is a logical counter for local events and message order. Increase it for a local event. On receipt, set it to max(local, sender) + 1.

### 12. Does a larger counter prove causality?

No. If one event causally precedes another, its Lamport value is smaller. The reverse is not sufficient. Use the same event ID and send/receive evidence to follow the message path.

### 13. Are Lamport clocks physical clock synchronization?

No. They do not synchronize UTC or measure elapsed time. Database time controls lease expiry. A multi-host physical clock synchronization study was not added.

### 14. What happens to a clock on restart?

The counter is stored in PostgreSQL for its node identity. Restarting the same identity resumes its counter. A replacement with a new identity has a separate counter.

### 15. What remains unchanged during an event retry?

The event ID, origin node, origin counter, and parent reference remain unchanged. A new publication claim records its own worker receive/send counters.

### 16. How do we implement mutual exclusion?

PostgreSQL account locks and advisory locks coordinate shared operations. Fixed account lock order reduces deadlock risk. This is database coordination, not a named fair distributed mutex algorithm.

### 17. What did the concurrency test establish?

With 10 units available, 20 competing debit attempts of 1 unit produced 10 posted and 10 rejected results. It also checked opposing transfers and same-key requests. Journals stayed balanced and reconciliation matched.

### 18. What is a lease?

A lease is ownership that expires unless renewed. The database uses its own time for expiry. A replacement can claim expired work.

### 19. Why do we also need fencing?

An old process can resume after its lease expires. An increasing generation lets the database reject that process's stale update. Ownership checks include owner, generation, and expiry.

### 20. Is our snapshot a Chandy-Lamport snapshot?

No. It is one repeatable-read PostgreSQL transaction. It excludes process state and messages in transit outside that database. Redis and service observations have separate collection times.

Give the short answer first. Add the evidence only when the examiner asks.

## Viva: Unit 4 and recovery

### 21. Why use Docker Compose?

It defines services, networks, volumes, and replica settings for a repeatable study. The mini-project requirement accepts Docker or Kubernetes. Compose fits this single-host demonstration.

### 22. Why did we not add Kubernetes?

The required Docker deployment already works. This study did not establish a need for cluster orchestration. Kubernetes can be a later study for multi-host scheduling and operational requirements.

### 23. How does API load balancing work?

A private Go gateway checks readiness and refreshes Docker DNS. It distributes requests among ready API instances. It preserves request and authorization context and does not automatically retry financial commands.

### 24. How can two APIs share safe command behavior?

Shared PostgreSQL controls coordinate idempotency, authorization-related replay controls, and financial updates. Per-process memory alone would not safely enforce a system-wide financial invariant.

### 25. What does FOR UPDATE SKIP LOCKED do?

It locks selected rows and skips rows already locked by another worker. This allows workers to claim different queued work. Stored lease and fencing checks protect updates after the claim transaction ends.

### 26. Is delivery exactly once?

No. Delivery is at least once. A crash after publication but before acknowledgment can cause a second delivery of the same event. Consumers use idempotency and aggregate-version checks.

### 27. How does scheduler election work?

Candidates coordinate through a PostgreSQL tenant lease. Only the current owner and generation can manage the scheduled job. A replacement takes over after expiry. This is not Bully, Ring, or Raft.

### 28. Can the scheduler move money?

No. Its restricted identity can create internal reconciliation evidence only for allowed tenants. It cannot submit transfers or read unrestricted base financial tables.

### 29. Can a scheduled report repeat?

Yes, if the report commits but its job acknowledgment is lost. The job interval remains unique and ownership is fenced. A repeated report does not create a repeated financial movement.

### 30. Is this blockchain or distributed ledger technology?

No. It is a ledger application with distributed service processes and one PostgreSQL authority. It has no blockchain consensus, blocks, or peer-replicated ledger network.

Give the short answer first. Add the evidence only when the examiner asks.

## Viva: results and limits

### 31. What does the performance test prove?

It compares selected replica configurations at a fixed demand on one host. Correctness checks passed. It does not measure maximum capacity or establish a proportional throughput increase.

### 32. What are p50 and p95?

p50 is the median measured response time. p95 is the value at or below which 95 percent of measured responses finish. They describe the measured workload, not every future request.

### 33. Why did throughput stay near 10.2 commands/s?

The clients offered a paced workload of roughly that rate. More replicas cannot show higher capacity when demand stays fixed. Database locks can also limit concurrent commands.

### 34. Why can more replicas increase latency?

They add connections, contention, and scheduling overhead. Short runs also contain measurement noise. The report retains both improvements and regressions.

### 35. What happens when one API fails?

The gateway excludes the failed instance after its check. An interrupted caller can safely repeat the same command with the same key. The recorded drill resolved 40 commands and reconciled.

### 36. What happens when PostgreSQL fails?

Authoritative financial commands and checks cannot complete safely. The system reports unavailability instead of assuming success from Redis. This study does not provide PostgreSQL high availability.

### 37. What happens when Redis fails?

The financial record remains in PostgreSQL. Projection and transport work can be delayed. The recorded snapshot check exercised cache loss and stale cache; the UI must show source and collection state honestly.

### 38. How did we check existing features?

The saved final validation records 42 passing Go packages, 214 frontend unit tests, 245 browser checks, and 6 real-stack journeys. These are recorded results for a captured source boundary, not new checks run to create this guide.

### 39. What are the main limits?

The study uses one host and one authoritative database. Delivery is at least once. The snapshot is database-only, benchmarks are short, and a Windows race check was not executed. These limits must stay in the presentation.

### 40. What would you add next?

Choose one measured need first. Possible studies include database failover, repeated saturation benchmarks, or multi-host deployment. Do not add Kubernetes, serverless, or blockchain only to increase the feature count.

All paths below are relative to D:\Work\Project\Dev\LedgerSync.

## Find the implementation

| Feature | Main implementation path |
| --- | --- |
| Study deployment | deploy/compose/docker-compose.study.yml<br>scripts/start-unit-study.ps1 |
| Private gateway | cmd/study-proxy/main.go<br>internal/platform/routing/ |
| Worker coordination | internal/application/lease/<br>internal/platform/db/outbox_repository.go<br>migrations/000041_worker_claim_fencing.up.sql |
| Lamport clocks | internal/platform/db/logical_clock_repository.go<br>internal/application/outbox/causal_publisher.go<br>migrations/000042_lamport_event_metadata.up.sql |
| Snapshot evidence | internal/application/reconciliation/snapshot.go<br>internal/platform/db/reconciliation_repository.go |
| Optional scheduler | cmd/reconciliation-scheduler/main.go<br>internal/application/scheduler/<br>migrations/000043_reconciliation_scheduler.up.sql |
| Developer UI | web/src/features/developer/DistributedSystems.tsx |
| Event order UI | web/src/features/operations/EventOrderEvidence.tsx |
| Snapshot UI | web/src/features/reconciliation/ReconciliationViews.tsx |

### Local source and evidence references

Implementation plan: docs/plans/ledgersync-unit-3-unit-4-implementation-plan.md. All 11 required steps are marked verified. Product release status remains separate from academic study completion.

Evidence folder: docs/release-evidence/unit-3-unit-4/. Read handover.md for commands, clock-rules.md for counter rules, scheduler.md for lease rules, performance-report.md for methods, and progress.md plus final-validation.md for verification history. Retained failed runs explain corrections.

Recorded evidence has its own source and image fingerprints. A fresh run can use a different working tree. Label it as a new run and preserve the original evidence. The downloaded web asset stays historical until it is regenerated and the web images are rebuilt.
