# Scaling and recovery measurements

Historical candidate evidence: retain this report for its recorded source and workload. It does not qualify the R09 source. See [the current local component study](../../portfolio/capacity.md) for its measured scope and limits.

## Method

- Source revision: `ed1662ebc52fbff49bd75bc957b5d2fa637fa73c`. The working tree contains changes.
- Source fingerprint: `8c791697df06c38a2deb98c8c1072e1a9ba688269f90e7403e1239e7ef558384`. Image IDs are in [matrix.json](performance/matrix.json).
- Docker: 29.8.2; 32 logical CPUs; 11.49 GiB engine memory.
- Each run uses a fresh database and cache. The seed signature is `6/3/2/4/260691419`.
- Warm-up: 5 seconds. Workload: 30 nominal seconds; 10 paced clients.
- The last client iteration has no trailing sleep. Use measured command throughput, not the configured client count, as the result.
- Independent traffic uses two disjoint account pairs in one tenant. Contended traffic uses one pair. Both retain normal authentication, authorization, rate limits, exact money, idempotency, balance reads, and periodic account/history reads.
- Each API and worker has one CPU, 256 MiB memory, and four database connections. Total limits increase with replicas. The web service and gateway retain fixed limits. PostgreSQL and Redis use the same host.
- The scheduler is disabled during each workload. The normal workspace remains running. Background container names are recorded.
- Each workload includes same-key replays. Logical command latency includes bounded conflict retries. The latency table includes successful replay calls.
- Resource and queue samples are collected approximately every five seconds. Peaks can occur between samples.
- Reconcile each ledger after queue drain. Reject a run if journals are duplicated or unbalanced, delivery stops, or a command has an unresolved result.

## Results

| Workload | API / worker | Posted/s | p50 ms | p95 ms | Business rejection | Retryable attempt | Unresolved error | Queue age s | CPU peak % | Memory peak MiB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| independent | 1 / 1 | 10.29 | 113.00 | 187.00 | 0 | 0 | 0 | 0.11 | 10.21 | 18.75 |
| independent | 2 / 1 | 10.28 | 101.00 | 192.30 | 0 | 0 | 0 | 0.09 | 9.55 | 27.50 |
| independent | 1 / 2 | 10.19 | 122.00 | 240.00 | 0 | 0 | 0 | 0.00 | 15.16 | 26.60 |
| independent | 2 / 2 | 10.25 | 106.50 | 194.55 | 0 | 0 | 0 | 0.16 | 14.55 | 35.35 |
| hot | 1 / 1 | 10.24 | 155.00 | 287.55 | 0 | 0 | 0 | 0.27 | 13.70 | 18.74 |
| hot | 2 / 1 | 10.28 | 105.50 | 207.20 | 0 | 0 | 0 | 0.31 | 9.35 | 27.36 |
| hot | 1 / 2 | 10.29 | 110.00 | 168.10 | 0 | 0 | 0 | 0.00 | 13.17 | 26.53 |
| hot | 2 / 2 | 10.28 | 113.50 | 214.10 | 0 | 0 | 0 | 0.16 | 22.34 | 35.20 |

CPU and memory peaks are combined API/worker samples. CPU100% is one core. The machine-readable comparison also records queue depth, publication throughput, replay counts, resource limits, and k6 threshold exit codes.

## Comparison

- independent, 2 API / 1 worker: p95 increased 2.83% from the one/one run. Posted throughput changed -0.15%.
- independent, 1 API / 2 worker: p95 increased 28.34% from the one/one run. Posted throughput changed -0.96%.
- independent, 2 API / 2 worker: p95 increased 4.04% from the one/one run. Posted throughput changed -0.37%.
- hot, 2 API / 1 worker: p95 decreased 27.94% from the one/one run. Posted throughput changed 0.43%.
- hot, 1 API / 2 worker: p95 decreased 41.54% from the one/one run. Posted throughput changed 0.55%.
- hot, 2 API / 2 worker: p95 decreased 25.54% from the one/one run. Posted throughput changed 0.44%.

## Correctness and limits

All eight runs passed balanced-journal, unique-journal, drained-delivery, command-count, and internal reconciliation checks. Raw k6 logs retain thresholds and request failures. A threshold failure is not removed from the record.

This is a short, fixed-demand comparison on one machine. It does not measure maximum capacity, statistical significance, sustained production load, or failure across machines. Small differences can be local noise. More replicas do not establish a proportional throughput increase. Account locks and the shared tenant policy lock still limit parallel financial commands, including commands on independent accounts in one tenant. Worker replicas improve recovery options; this small workload can be served by one worker. PostgreSQL remains a common dependency.

## Recovery evidence

See [API failure](api-failure.json), [worker process crashes](worker-crash-verified.txt), and [scheduler takeover](scheduler-takeover.json). Failure detection includes container command and probe overhead. Record the final rerun commands and logs in the progress record. Delivery remains at least once.

Measured API detection: 1661.84 ms. Worker recovery: before_publish 620 ms; after_publish 615 ms. Scheduler takeover: stop 4901 ms; pause 5025 ms. See [recovery-report.json](recovery-report.json). Worker timing includes a deliberate 600 ms wait.
