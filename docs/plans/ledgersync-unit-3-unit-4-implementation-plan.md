# LedgerSync: Unit 3 and Unit 4 implementation plan

**Date:** 2026-10-10  
**Status:** Complete. All 11 required steps are verified. See the evidence record.
**Purpose:** Add useful distributed system functions. Demonstrate scalability and communication between services.  
**Writing style:** Follow ASD-STE100 principles. Use short sentences, active voice, and defined technical terms.

## 1. Project outcome

Build on the existing LedgerSync application. Keep the financial record in PostgreSQL. Use the existing Go API, worker, Redis cache, and Next.js console.

At completion, demonstrate these functions:

1. Distribute API requests between two application instances.
2. Process committed events with two worker instances.
3. Record causal event order with Lamport clocks.
4. Control simultaneous transfers with database locks.
5. Produce a consistent internal reconciliation report.
6. Select one reconciliation scheduler and replace it after failure.
7. Record performance, correctness, and recovery results.

The course requirement accepts Docker or Kubernetes. Use Docker Compose for the required work. Consider Kubernetes only after the Docker results identify a useful need.

This is an academic extension plan. The [master delivery register](ledgersync-master-progress.md) remains the authority for product and release status.

## 2. Current functions and required additions

| Area | Function found in the current code | Required addition |
|---|---|---|
| Deployment | Docker Compose starts the web service, API, worker, PostgreSQL, and Redis. | Add an isolated deployment configuration for replica tests. |
| Service communication | The web service calls the private API. The worker reads committed events and updates downstream systems. | Record evidence from a complete request and event path. |
| Worker coordination | PostgreSQL selects events with `FOR UPDATE SKIP LOCKED`. Claims have an owner and an expiry time. | Verify parallel operation. Protect acknowledgments against expired or replaced claims. |
| Mutual exclusion | Account operations use database locks. Reconciliation uses an advisory lock. | Add controlled concurrency tests and record their results. |
| Reconciliation | A repeatable-read transaction records a database snapshot and a schema version. | Present the snapshot boundary and report delivery work separately. |
| Event ordering | Events have identifiers, timestamps, and aggregate versions. | Add explicit Lamport send and receive rules. |
| Scheduling | This review did not establish an automatic scheduler with leader failover. | Add an optional scheduler with a safe lease and takeover rules. |
| Performance | A k6 transfer workload and earlier performance evidence exist. | Extend the workload. Measure the changed system with one and two replicas. |

Source files reviewed include `deploy/compose/docker-compose.yml`, `internal/platform/db/outbox_repository.go`, `internal/platform/db/reconciliation_repository.go`, and `cmd/outbox-worker/main.go`.

## 3. Terms

| Term | Meaning in this plan |
|---|---|
| Replica | Another running instance of the same application service. |
| Load balancer | A service that distributes requests between application instances. |
| Lamport clock | A logical counter that records causal order through local events and received messages. |
| Claim | A temporary right to process a queued event. |
| Lease | A right that expires unless its owner renews it. |
| Fencing token | An increasing value that lets the database reject work from a previous owner. |
| Snapshot | A consistent view of database records within a transaction. |
| Idempotency | Repeating the same command does not create another financial movement. |
| p95 response time | The response time at or below which 95 percent of measured requests finish. |

## 4. Rules for all steps

1. Preserve existing user records and unrelated code changes.
2. Use an isolated Compose project and separate data volumes for demonstrations.
3. Use synthetic accounts and test amounts in that environment.
4. Keep PostgreSQL as the financial authority.
5. Preserve exact amounts, immutable postings, authorization, and idempotency.
6. Keep database and cache services private.
7. Add schema changes through migrations. Assess compatibility with existing records and workers.
8. Do not put secret values, tokens, or customer data in evidence files.
9. Write failure tests before changes to financial or coordination behaviour.
10. Stop acceptance if a duplicate movement, unbalanced journal, tenant leak, or unexplained mismatch occurs.
11. Record failed tests and failed requests. Do not remove them to improve reported results.
12. Verify each step before the next dependent step starts.

## 5. Implementation sequence

### Step 1 — Record the baseline

**Topics:** Distributed web systems; communication between services.

**Actions:**

1. Record the Git commit and the existing working changes.
2. Record the service images, schema version, and machine resources.
3. Create an isolated test environment through the existing local acceptance tools.
4. Check account creation, funding review, transfers, corrections, approvals, events, and internal reconciliation.
5. Check the sidebar, navigation, exports, and unavailable-data states.
6. Save the commands, results, and environment details in an evidence directory.

**Acceptance:** The baseline workflows pass. The normal workspace data is unchanged.

**Output:** A baseline record and a repeatable test setup. Store new evidence under `docs/release-evidence/unit-3-unit-4/`.

### Step 2 — Prepare replica-safe configuration

**Topics:** Distributed web systems; scalability.

**Dependency:** Step 1.

**Actions:**

1. Add a separate Compose configuration for the academic environment.
2. Give each API and worker instance a unique identity.
3. Share the required signing configuration through the existing secret mechanism.
4. Check session validation, replay protection, idempotency, and rate limits across API instances.
5. Correct any control that depends on one process when its contract requires a shared limit or state.
6. Set database connection limits for the total number of replicas.
7. Keep migrations as one controlled setup operation.

**Acceptance:** A request can reach either API instance without a change to its authorization or financial result. Repeated commands still create at most one movement.

**Output:** Replica configuration and tests for controls shared between instances.

### Step 3 — Add API load balancing

**Topics:** Distributed web systems; Docker deployment; scalability.

**Dependency:** Step 2.

**Actions:**

1. Add a private load balancer, such as NGINX, to the academic environment.
2. Route the Next.js backend requests through that load balancer.
3. Configure the load balancer to discover replica addresses after an instance restarts.
4. Use readiness checks before sending traffic to an instance.
5. Preserve actor assertions, request identifiers, and correlation identifiers.
6. Verify streaming connections, live investigation connections, and timeout behaviour where these paths use the balancer.
7. Stop one API instance during a controlled workload.
8. Record interrupted requests, retries, recovery time, and final financial results.

**Acceptance:** Both API instances serve requests. Requests continue through a healthy instance after failure detection. A retry cannot duplicate a transfer.

**Output:** A load balancer configuration and a failure demonstration.

### Step 4 — Verify parallel workers and safe claims

**Topics:** Mutual exclusion; distributed processing; recovery.

**Dependency:** Step 2. This step can run alongside Step 3.

**Actions:**

1. Run two instances of the existing worker.
2. Verify unique identities for event publishing, cache projection, and webhook work.
3. Review claim expiry, renewal, acknowledgment, and retry behaviour.
4. Add a new claim token or generation for each claim. Require it for completion, retry, and failure updates.
5. Use PostgreSQL time for shared claim expiry. Renew long tasks and keep batch sizes within the lease limit.
6. Reject an acknowledgment from an expired or replaced claim.
7. Stop a worker after it claims an event.
8. Verify that another worker can recover the event after expiry.
9. Stop a worker after publication but before acknowledgment.
10. Verify that repeated delivery does not repeat a financial effect or replace a newer cache version.
11. Check pending Redis stream messages and their recovery path.

**Acceptance:** Work is shared between workers. Abandoned claims recover. Stale owners cannot change the current claim result. Duplicate deliveries remain safe.

**Output:** Worker coordination tests and recovery evidence.

**Limit:** Delivery can occur more than once. Do not report exactly-once delivery from a claim test. Use idempotent consumers and preserve event identifiers.

### Step 5 — Add Lamport event metadata

**Topics:** Logical clocks; Lamport's algorithm.

**Dependency:** Step 4.

**Actions:**

1. Define a compatible event envelope with a node identity, event identity, parent reference, and logical timestamp.
2. Define each participant's stable node identity, clock, and persistent state. Include API and worker instances.
3. Increase the local counter before recording a local event or sending a message.
4. On receipt, set the counter to `max(local counter, received counter) + 1`.
5. Make concurrent counter updates safe. Restore the counter before a node uses its existing identity after restart.
6. Store the committed event timestamp with its existing outbox transaction.
7. Preserve that timestamp and event identity during retries. Record delivery attempts separately.
8. Accept older event envelopes that lack the new fields. Label their causal metadata as unavailable.
9. Use the logical timestamp for investigation evidence. Preserve aggregate versions for cache ordering and PostgreSQL for posting decisions.
10. Test delayed messages, retries, concurrent events, counter overflow, and process restart.

**Acceptance:** A receive timestamp exceeds the corresponding send timestamp. Retries preserve the original event envelope. Clock metadata does not change financial outcomes.

**Output:** Clock rules, event contract changes, tests, and a causal event trace.

**Limit:** A larger Lamport value does not prove that one event caused another. A database sequence alone does not implement these message rules.

### Step 6 — Demonstrate mutual exclusion

**Topics:** Mutual exclusion; concurrency control; deadlock handling.

**Dependencies:** Steps 3 and 4.

**Actions:**

1. Reuse the existing financial command and account-locking path.
2. Send concurrent debits against the same test account through different API instances.
3. Send opposite-direction transfers between the same two accounts.
4. Check consistent lock order, bounded waiting, and deadlock handling.
5. Send the same idempotency key through both API instances.
6. Submit two reconciliation requests for the same tenant at the same time.
7. Record accepted commands, rejected commands, and final posting totals.

**Acceptance:** No overspend or duplicate movement occurs. Each committed journal is balanced. Reconciliation reports no unexplained mismatch. Lock failures produce controlled results.

**Output:** Concurrent transfer and reconciliation evidence.

**Limit:** Describe this as database-coordinated mutual exclusion. Do not name a different distributed algorithm unless that algorithm is implemented and tested.

### Step 7 — Extend snapshot and delivery evidence

**Topics:** Global state; consistency; distributed database trade-offs.

**Dependencies:** Steps 4 and 6.

**Actions:**

1. Preserve the existing repeatable-read reconciliation transaction.
2. Confirm the snapshot watermark, schema version, account scope, and posting checks.
3. Record database-backed outbox counts in the same snapshot when suitable.
4. Show separately sampled Redis and worker observations with their collection times.
5. Keep ledger posting status separate from downstream delivery status.
6. Test reconciliation while transfers and event delivery continue.
7. Test cache loss, stale cache versions, and database unavailability.
8. Verify database fallback where the current read contract permits it. Otherwise, return an explicit unavailable result.

**Acceptance:** The ledger report uses one consistent database snapshot. The UI identifies pending delivery and unavailable observations correctly.

**Output:** Snapshot contract, consistency tests, and a report example.

**Limit:** This is a database snapshot with service observations. It is not a Chandy-Lamport snapshot of all processes and channels.

### Step 8 — Add reconciliation scheduler failover

**Topics:** Leader selection; leases; failure recovery.

**Dependencies:** Steps 6 and 7.

**Actions:**

1. Add an optional local scheduler. Keep scheduled operation disabled by default.
2. Run two scheduler instances when the academic profile is selected.
3. Use a short PostgreSQL transaction to acquire or renew the scheduler lease.
4. Use database time for lease expiry. Increase the fencing token when ownership changes.
5. Verify the current owner and token when a scheduled job is created.
6. Enforce one scheduled job for each tenant and scheduled interval.
7. Release the scheduling transaction before a reconciliation run starts.
8. Use the existing tenant reconciliation lock when the job runs.
9. Stop scheduling immediately if lease renewal fails.
10. Stop or pause the leader. Verify takeover and rejection of work from the previous owner.
11. Restrict the scheduler identity to the required tenants and actions.

**Acceptance:** A valid owner creates scheduled jobs. A replacement takes over after lease expiry. A stale owner cannot create another job for the same interval. Manual reconciliation remains usable.

**Output:** Lease migration, scheduler, failover tests, and measured takeover time.

**Limit:** This is database-coordinated leader selection. PostgreSQL remains a dependency. It does not establish a Bully, Ring, or consensus algorithm implementation.

### Step 9 — Present evidence through the existing UI

**Topics:** Operator usability; technical demonstration.

**Dependencies:** Steps 5, 7, and 8.

**Actions:**

1. Add a collapsed section named **Distributed systems** inside the existing Developer page.
2. Show replica count, worker progress, scheduler state, and links to test evidence when those values are available.
   Use labels such as **Event order**, **Delivery attempts**, and **Active scheduler**.
3. Put node identity and logical timestamps in event details.
4. Put the snapshot boundary in reconciliation details.
5. Keep technical explanations in expanded details or the Guide.
6. Use explicit labels for historical, unavailable, and pending evidence.
7. Run load tests and failure drills from scripts. Do not add disruptive test buttons to normal operator pages.
8. Check keyboard access, mobile layout, screen reader labels, and existing navigation.
9. Use the current design tokens and shared UI components.

**Acceptance:** Normal financial tasks remain easy to find. Technical evidence is available without adding a new primary navigation item. The existing pages retain their functions.

**Output:** Focused UI changes and desktop and mobile review evidence.

### Step 10 — Measure scaling and recovery

**Topics:** Scalability; communication; performance; resilience.

**Dependencies:** Steps 3 through 9.

**Actions:**

1. Extend the existing k6 workload. Keep authentication and normal command validation active.
2. Record a baseline with one API and one worker.
3. Repeat with two APIs and one worker.
4. Repeat with one API and two workers.
5. Repeat with two APIs and two workers.
6. Use the same data size, workload, machine resources, warm-up, and measurement duration.
   State whether the total service resource limits change between runs.
7. Run separate workloads for independent accounts and contention on one account.
8. Record throughput, p50 and p95 response times, error counts, queue age, and resource use.
9. Separate expected business rejections from technical failures.
10. Run the approved failure scenarios and record recovery time.
11. Reconcile the test ledger after every workload and recovery drill.
12. Report increases, unchanged results, and regressions. Explain any database, lock, or machine limit.

**Acceptance:** Results are repeatable. Correctness checks pass for every accepted run. The report states where replicas help and where they do not help.

**Output:** Machine-readable results, comparison tables, and a measured conclusion.

### Step 11 — Complete regression checks and handover

**Dependency:** Step 10.

**Actions:**

1. Run relevant Go unit, integration, contract, and fault checks for the changed code.
2. Run the existing financial journey and local acceptance checks in the isolated environment.
3. Run frontend lint, type, build, and applicable browser checks.
4. Compare the account, funding, transfer, approval, correction, event, and reconciliation workflows with Step 1.
5. Verify upgrade compatibility with older events and existing records.
6. Document how to disable the scheduler and remove the academic configuration.
7. Use forward-compatible migration recovery. Do not delete committed financial records during rollback.
8. Remove only the isolated test resources after their paths and project identity are verified.
9. Check the normal LedgerSync service health.
10. Update the completion table below with evidence links.

**Acceptance:** Existing workflows pass. Required evidence is present. The normal workspace is healthy.

**Output:** Final validation record and demonstration instructions.

## 6. Optional work after the required steps

These items are outside the required implementation sequence. Add one only when the course or a measured use case needs it.

| Topic | Proposed work | Entry condition |
|---|---|---|
| Kubernetes | Deploy the existing services to a local cluster. Add Services, probes, resource limits, persistent storage, and replica tests. | The Docker results are complete. Record the need and operating cost before adding the cluster. |
| Physical clock synchronization | Configure host NTP and record clock offsets. Use monotonic time for local elapsed-time measurements. | The demonstration uses separate machines. |
| Vector clocks | Detect concurrent investigation-note updates in a separate demonstration. | Independent writers are part of the selected use case. |
| Chandy-Lamport global snapshot | Implement marker messages and channel recording in a controlled node simulation. | Full algorithm coverage is required. Define FIFO channel assumptions and failure limits. |
| Bully or Ring election | Demonstrate the named algorithm with test nodes and recorded messages. | The instructor requires that algorithm. Keep it separate from financial authority. |
| Beacon protocol | Implement the exact time synchronization protocol taught in class. | The instructor supplies the protocol definition. |
| Fair mutual exclusion | Simulate the Lodha and Kshemkalyani algorithm. Measure acquisition order and waiting time. | The self-study topic requires a working demonstration. |
| Distributed deadlock detection | Compare Knapp's classification and demonstrate a selected detection approach. | The report needs this self-study extension. |
| Serverless processing | Choose AWS Lambda or Cloudflare Workers for a bounded evidence-file or notification task. | The use case, account, credentials, cost, and data boundary are defined. |
| Distributed file storage or Hadoop | Process large synthetic audit archives through a separate storage or batch workload. | Measurements show a dataset or throughput need. |
| Blockchain, remote objects, or Megaport | Prepare a separate design or case study with a specific problem. | The course requires the topic or independent participants need the proposed function. |

Do not count object storage as a distributed file system. Do not count an append-only ledger or a hash as a blockchain. Do not count ordinary RPC as a complete remote object model.

## 7. Course evidence and completion record

| ID | Required step | Unit or requirement | Status | Evidence |
|---|---|---|---|---|
| U01 | Baseline and isolated test setup | Mini-project deployment | Verified | [Baseline evidence](../release-evidence/unit-3-unit-4/progress.md) |
| U02 | Replica-safe configuration | Unit 4; scalability | Verified | [Replica evidence](../release-evidence/unit-3-unit-4/replica-configuration.md) |
| U03 | API load balancing | Unit 4; service communication | Verified | [API failure evidence](../release-evidence/unit-3-unit-4/api-failure.json) |
| U04 | Parallel workers and recovery | Unit 3; mutual exclusion | Verified | [Worker evidence](../release-evidence/unit-3-unit-4/parallel-workers.json) |
| U05 | Lamport event metadata | Unit 3; logical clocks | Verified | [Clock rules](../release-evidence/unit-3-unit-4/clock-rules.md), [live trace](../release-evidence/unit-3-unit-4/causal-trace.json) |
| U06 | Concurrent transfer demonstration | Unit 3; mutual exclusion | Verified | [Concurrency results](../release-evidence/unit-3-unit-4/concurrent-transfers.json) |
| U07 | Snapshot and delivery evidence | Unit 3 global-state concepts; Unit 4 consistency | Verified | [Snapshot report](../release-evidence/unit-3-unit-4/snapshot-report.json) |
| U08 | Scheduler takeover | Unit 3; leader selection | Verified | [Scheduler rules](../release-evidence/unit-3-unit-4/scheduler.md), [takeover results](../release-evidence/unit-3-unit-4/scheduler-takeover.json) |
| U09 | Clear technical evidence UI | Project usability | Verified | [UI checks](../release-evidence/unit-3-unit-4/ui-review-fix-verified.txt), [design review](../release-evidence/unit-3-unit-4/ui-design-review.md) |
| U10 | Scaling and recovery results | Mini-project scalability | Verified | [Measurements](../release-evidence/unit-3-unit-4/performance-report.md), [recovery timing](../release-evidence/unit-3-unit-4/recovery-report.json) |
| U11 | Regression checks and handover | Existing feature protection | Verified | [Final validation](../release-evidence/unit-3-unit-4/final-validation.md), [handover](../release-evidence/unit-3-unit-4/handover.md) |

Use **Planned**, **In progress**, **Verified**, or **Blocked**. Mark a step **Verified** only after its acceptance conditions pass.

For each demonstration, record the source revision, local change identifier, image identifiers, environment, commands, workload, results, and limits. A test on one machine demonstrates process and service distribution. It does not establish availability across multiple machines.

## 8. Suggested report structure

1. State the ledger use case and the project objective.
2. Describe the existing service architecture.
3. Map each implemented addition to its syllabus topic.
4. Explain the event, locking, snapshot, and lease rules.
5. Describe the test environment and measurement method.
6. Present performance and failure results.
7. State the limits and remaining work.
8. Link each claim to its evidence.

Use this project description only after the required demonstrations pass:

> LedgerSync is a containerised distributed ledger application. It demonstrates service communication, concurrent transaction control, causal event ordering, background processing, and service scaling.

## 9. References

- [LedgerSync architecture](../architecture.md)
- [LedgerSync constitution](../../.specify/memory/constitution.md)
- [Local runtime guide](../runbooks/local-runtime-smoke.md)
- [Lamport: Time, Clocks, and the Ordering of Events in a Distributed System](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/12/Time-Clocks-and-the-Ordering-of-Events-in-a-Distributed-System.pdf)
- [Chandy and Lamport: Distributed Snapshots](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/12/Determining-Global-States-of-a-Distributed-System.pdf)
- [PostgreSQL locking](https://www.postgresql.org/docs/current/explicit-locking.html)
- [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- [Docker Compose networking](https://docs.docker.com/compose/how-tos/networking/)
- [Kubernetes leases](https://kubernetes.io/docs/concepts/architecture/leases/)
