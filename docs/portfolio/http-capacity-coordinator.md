# HTTP and BFF capacity comparison

Status: source implemented and verified with offline tests. Actual container execution and capacity acceptance remain **not run**. Docker, normal services and deployments must stay stopped during this repair pass. The [repair register](audit-remediation.md) states the current phase result. Old component measurements keep their original scope and identity.

## Purpose and scope

This tool measures a complete request through the browser-facing server, or BFF, and the private API. It compares one API with two APIs. It separates a fair comparison with the same total resources from a comparison that supplies more resources. A higher replica count alone does not prove an improvement.

The coordinator is [run-http-capacity.mjs](../../scripts/portfolio/run-http-capacity.mjs). It runs cases one after another. It does not publish images or contact a deployment provider. The default command only prints the plan:

```powershell
node scripts/portfolio/run-http-capacity.mjs --preview
```

To inspect one case without starting anything:

```powershell
node scripts/portfolio/run-http-capacity.mjs --preview --case=hot-pair/two-api-fixed-total/1
```

Preview writes no file and calls no Docker command. Do not set the execution approval variable during this repair pass. A future local run requires an explicit runtime decision, `LEDGERSYNC_CAPACITY_RUNTIME_APPROVED=true`, and either `--execute --case=<exact-case-id>` or `--execute --all`. The CLI, coordinator and concrete adapter each check their execution boundary. The environment variable records a caller decision; it cannot grant permission by itself.

## Comparison cases

There are 27 cases: three traffic profiles, three API allocations and three repeats. The case ID has the form `profile/allocation/repeat`.

| Part | Values and meaning |
|---|---|
| Profile | `independent-pairs`: alternate two pairs in one tenant. `hot-pair`: use one pair. `multi-tenant`: alternate pairs in two tenants. |
| API allocation | `one-api`: one API with 1 CPU and 512 MiB. `two-api-fixed-total`: each API has 0.5 CPU and 256 MiB. `two-api-per-replica`: each API has 1 CPU and 512 MiB. |
| Repeat | `1`, `2`, `3`. Keep every repeat, including a poor result. |

The following budgets stay fixed in every case. MiB is a memory unit of 1,048,576 bytes.

| Service | Total CPU | Total memory |
|---|---:|---:|
| Worker | 1 | 512 MiB |
| PostgreSQL | 1 | 768 MiB |
| Redis | 1 | 256 MiB |
| Two BFFs together | 1 | 768 MiB |
| Private request router | 0.25 | 128 MiB |
| Telemetry collector | 0.25 | 384 MiB |

Each BFF has a separate server-selected tenant. Each has half the shared BFF budget. This uses the existing login boundary. It does not accept a tenant chosen by the browser. The router sends each request to one API. It does not retry a financial write after an uncertain transport result. The client owns the same-key retry.

The API connection-pool total is four for the baseline and the fixed-total case. In the per-replica case, each API has four. The worker has two. Fixture and measurement access use a separate administrative connection only in the temporary database. The one-time migration and seed containers have explicit limits too.

## Synthetic work and independent checks

Each case starts with eight accounts in two synthetic tenants. Six accounts start with 1,000,000 INR minor units each. Two accounts start with zero. A normal command transfers 100 minor units. No active hold, bank instruction, external statement or customer data is part of this workload.

Every tenth selected command repeats the prior key, reference and complete body. A fixed insufficient-funds command checks a durable business rejection. The client saves the recovery intent before it submits the transfer. It uses the actual local sign-in, opaque cookie, CSRF token, origin check and BFF routes. Cookies and credentials remain private.

The driver keeps an independent integer model. A successful receipt must match the intended status, currency and exact amount. One original key must always return the same receipt. The model applies each new posted intent once. It predicts all eight balances and their versions without reading production ledger calculations. Final read-only SQL compares those predictions with the actual balances and versions. It also checks balanced two-entry journals, outcome counts, settled keys, tenant boundaries, zero active holds and a drained publication queue. Internal reconciliation must match for each tenant. A balanced but incorrect amount cannot pass the model check.

## Load, measurements and recovery

1. Check foreign-account denial for both tenants.
2. Warm up at five offered commands per second for ten seconds.
3. Run separate ten-second calibration stages at five, ten and twenty commands per second. Keep the raw stages. They do not qualify sustained capacity. Recover original unknown results and drain the calibration backlog before the steady interval. Retain that queue checkpoint.
4. Offer ten commands per second for 300 seconds. Allow eight concurrent commands and at most 32 queued offers. Record dropped offers and generator delay. Measure command latency from its intended arrival time, including queue time, intent preparation and submission. Record service time separately.
5. Sample CPU, container working set, summed process RSS, driver/host load, pool waits, policy-lock metrics, database size, connections, lock waiters and publication depth/age. Sampling uses asynchronous commands so it does not block the driver. Missing evidence stays unavailable. It does not become zero.
6. Stop further offers if a sample exceeds the selected queue/database budget. Drain current callers and retain the short interval. A financial mismatch stops the case and prevents qualification.
7. Discard one successful response body, restart the exact owned API, and recover the receipt with the original key. This is a lost-body test. The separate R07 integration test covers an actual post-commit connection abort.
8. Pause the exact owned Redis container during a separate five-second stage. Unpause it, drain callers, recover original unknown outcomes, wait for readiness and drain publication. Keep actual recovery duration.
9. Compare independent balances and internal reconciliation. Capture final enforcement evidence. Verify cleanup before a case can enter a comparison summary.

The workload has a 600-second case bound and a 6,000-sequence ceiling. Cleanup has a separate 120-second command deadline. The CLI has a six-hour bound. A bound or cancellation failure retains an incomplete record. It does not qualify. Sample intervals include probe cost; these finite observations do not establish CPU or memory peaks. Summed RSS can count shared pages more than once.

The unchanged local thresholds are: command p95 at most 500 ms, generator delay at most 500 ms, no dropped offer, no technical failure, oldest pending at most five seconds, pending depth at most 1,000, no dead event and database size at most 256 MiB. The full steady interval and complete resource evidence are required. Report new posted results, replays, business rejections, HTTP attempt failures and unresolved commands separately. Do not add failed HTTP attempts to failed command counts.

## Preservation and evidence

A future execution first freezes source and records its manifest, Git/index identity, machine identity and existing container identities. It builds unique local API, worker, web and database-tooling images. PostgreSQL, Redis and collector inputs use pinned digests. Every case uses those seven immutable image IDs and fresh secrets. Each case has a unique project, fresh bounded temporary storage and new loopback BFF ports. API, PostgreSQL, Redis, router and collector ports stay private.

Before a target action, check its exact container ID, immutable image, source/project/service labels, CPU and memory limits, read-only root, dropped capabilities, temporary mounts, approved read-only inputs and port allocation. A changed target is not an owned target. Cleanup refuses it, records the failure and continues with other independently verified targets. It never uses a general prune or normal-volume removal. Images remain under their unique local tags. No normal financial query, seed or migration is part of this coordinator.

Private records go under `data/portfolio/r09-YYYYMMDD/candidate-<timestamp>-<suffix>/`. Keep source, image and machine records, stage results, expected and observed financial results, resource samples, enforcement records, bounded redacted logs, failures and cleanup. Secret environment and resolved private Compose files are excluded from the public artifact manifest. They still remain private. Do not publish the directory. Per-case retained evidence is capped at eight MiB, excluding the frozen source/build inputs and the two private secret files. Log truncation is explicit.

The file verifier checks hashes, safe paths, expected service limits, image/source bindings, exact finances, complete cleanup and raw-measurement accounting. The runtime writes a complete comparison summary only after all 27 cases and their artifacts pass verification. An unfavourable performance result remains visible and cannot become a passing capacity result. A fixture can test the verifier; it cannot establish runtime provenance. A maintainer must inspect the real host and capture origin before publication.

## Implementation and exit condition

| File | Responsibility |
|---|---|
| [definition.mjs](../../scripts/portfolio/http-capacity/definition.mjs) | Cases, fixture identities and deterministic original/replay intent. |
| [coordinator.mjs](../../scripts/portfolio/http-capacity/coordinator.mjs) | Serial execution, partial ownership, failure retention and cleanup order. |
| [compose.mjs](../../scripts/portfolio/http-capacity/compose.mjs) | Disposable topology, fixed budgets, synthetic seed and private telemetry configuration. |
| [driver.mjs](../../scripts/portfolio/http-capacity/driver.mjs) | Real BFF HTTP, bounded offered load, original-key recovery and independent balances. |
| [router.mjs](../../scripts/portfolio/http-capacity/router.mjs) | Private request distribution with no automatic write retry. |
| [boundary.mjs](../../scripts/portfolio/http-capacity/boundary.mjs) | Exact owned-target and financial guards. |
| [measurement.mjs](../../scripts/portfolio/http-capacity/measurement.mjs) | One shared set of qualification rules. |
| [queries.mjs](../../scripts/portfolio/http-capacity/queries.mjs) | Read-only queue/resource and financial queries. |
| [runtime.mjs](../../scripts/portfolio/http-capacity/runtime.mjs) | Future approved local Docker adapter and evidence capture. |
| [evidence.mjs](../../scripts/portfolio/http-capacity/evidence.mjs) | Independent bounded artifact and report verification. |

The four `http-capacity*.test.mjs` files and the comparison tests run in the existing quality workflow. Native HTTP fixtures prove driver behavior. Mocked lifecycle tests prove cleanup decisions. Neither runs the real container topology, collector configuration or financial SQL. Those remain explicit first-rehearsal checks. Run one case first in a later approved window. Repair any runtime defect, freeze a new candidate, then collect all repeats with identical source and images. No new HTTP/BFF throughput, scaling, reservation or production-SLA claim is permitted until that acceptance work passes.
