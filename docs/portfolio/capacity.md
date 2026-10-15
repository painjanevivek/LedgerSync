# Local component capacity and recovery

Qualified locally on 15 October 2026. Use this result only for its stated configuration. It is a component study with synthetic USD records. It does not qualify HTTP, the Next.js BFF, candidate API images, a cloud provider, external funds or a production service target.

## Result

Three separate runs passed a requested 300-second steady interval at 10 offered commands per second. Each then passed a 60-second recovery stage at five commands per second. The recovery stage paused its own Redis container and killed and restarted its own native worker process. The final check compared exact balances, versions and journals with the independent model, ran internal reconciliation, waited for Redis projection convergence, and verified cleanup.

| Repeat | Steady commands | New posted transfers | Command p50 / p95 / p99, ms | Steady oldest pending, s | Recovery oldest pending, s | Total journals |
|---|---:|---:|---|---:|---:|---:|
| 1 | 3000 | 2640 | 17.84 / 27.27 / 33.74 | 0.42 | 28.00 | 3168 |
| 2 | 3000 | 2640 | 16.29 / 27.63 / 38.01 | 0.41 | 6.00 | 3168 |
| 3 | 3000 | 2640 | 15.20 / 27.49 / 95.11 | 2.10 | 5.80 | 3168 |

Latency covers all submitted financial commands. It includes replay and rejected responses. In each 3,000-command steady stage, 2,640 commands created new postings, 300 returned replay responses, and 60 created new rejected outcomes. There were 120 rejected responses in total because 60 replays returned a prior rejection. Do not add rejected-response counts to replay counts. Every qualified run had zero technical failures and zero dropped commands.

Ten offered commands per second therefore means 8.8 new posted transfers per second for this mix. Each fifth command also reads the authoritative debit balance and checks its required version. That read is reported separately. The scheduler uses fixed arrival times; it does not wait for the previous response before offering the next command. Eight bounded workers and a 32-item input queue limit the driver. A separate 48-command response-coupled burst is reported as a transient case.

## What changed and what did not pass

The measured delivery queue grew while financial command latency stayed low. The original adapter created and cleared a separate scoped transaction for each logical receipt. The candidate reuses one transaction for each consecutive group from the same tenant, with a maximum of 100 messages per call. It keeps transport order. Every receipt still calls the same controlled function. A failed tenant group rolls back. Earlier tenant groups can already have committed. Redis messages remain pending until receipt storage and cache processing succeed.

Publication preparation also reads the optional diagnostic origin in its existing scoped query. This removes one round trip. No migration, financial writer, policy limit, monetary lock, audit requirement or claim-generation check changed.

The following short calibrations used the same 20-command arrival profile and the same container limits. Each cohort has three runs with identical source and configuration within that cohort. The table shows commands offered during the final short steady stage and its sampled maximum queue age. The normal five-second age limit stopped a stage early when exceeded.

| Configuration | Final-stage commands, repeats 1 / 2 / 3 | Oldest pending seconds, repeats 1 / 2 / 3 |
|---|---|---|
| Original receipt adapter; batch 10 | 101, 121, 101 | 5.02, 5.30, 5.03 |
| Grouped receipts; batch 10 | 161, 181, 161 | 5.15, 5.15, 5.15 |
| Grouped receipts; batch 50 | 200, 181, 200 | 4.65, 5.00, 4.65 |

Grouping reduced queue growth in this bounded comparison. The selected 20-command rate still has no sustained qualification. A batch of 50 stayed within the age budget in two short intervals and exceeded it in the third. Default batch 10 remains. Do not turn the short result into a higher throughput or replica-scaling claim.

## Workload and limits

- Six synthetic customer accounts in two tenants. Intended account-pair weights are 6/12 for one hot pair, 3/12 for another pair in the same tenant, and 3/12 for the second tenant. The retained stage report contains the actual shape counts after replay mapping.
- One minor unit for normal commands. A fixed insufficient-funds intent creates the expected durable rejection. Every tenth selected command reuses the prior key and full intent.
- A 48-command burst, queue drain, five-second warmup, staged arrival rates, steady load, real recovery faults, final drain and independent financial checks. Each exact stage duration and offered count is retained.
- PostgreSQL: one CPU, 768 MiB, temporary storage capped at 512 MiB. Redis: one CPU, 256 MiB, temporary storage capped at 128 MiB. Each has one newly assigned loopback port. Neither mounts normal volumes.
- Go driver and native worker each use GOMAXPROCS=2. Configured pool maxima are API 4, worker 2, measurement/fixture 2 and reconciliation 2. The report samples actual connections and lock waiters.
- The normal budget is command p95 at most 500 ms, oldest pending at most five seconds, pending depth at most 1,000, no dead events, no driver drops, and database size at most 256 MiB. The controlled recovery stage permits queue age up to 60 seconds. These are local test budgets, not customer commitments.
- A host run has a 600-second wall bound. Its capacity test has a nine-minute test timeout and a smaller internal deadline. Output, claims and generated operations are bounded. An incomplete or skipped named test cannot qualify.
- Queue statistics are sampled once per second. Container resource samples are separate. Driver CPU and current Go heap are recorded. Native worker CPU and resident memory are not measured here. The final image qualification must inspect service resources separately. A sampled zero is not proof of no lock wait or no transient backlog.
- Fault records contain requested and actual timestamps. In the first qualified run, the requested three-second Redis pause lasted about 5.2 wall seconds. The worker returned ready about 11.0 seconds after its kill request. Do not report requested durations as measured durations.

## Slow receiver check

An additional integration check used a real signed TLS request held open for at least 1.2 seconds. Twelve postings ran with four callers while that request waited. Posting p95 was 170.64 ms. The worker renewed its one-second lease, and its one-connection pool served another query while HTTP was blocked. Reconciliation and exact financial conservation passed. A test-only injected transport routes the synthetic public-looking destination to the owned TLS receiver. Production destination checks have no loopback exception. This is one receiver and a finite burst; it is not a receiver-capacity claim.

## Run it safely

Use the host runner from the repository root:

```powershell
node scripts/portfolio/run-capacity.mjs --batch=10 --rate=10 --steady=300
```

Use `--steady=10` only for calibration. Allowed rate choices are 10, 20 and 30. Allowed batch choices are 10, 25 and 50. A short run always reports `capacity_qualified:false`, even when its correctness checks pass. A requested full qualification that exceeds its performance budget exits nonzero. Keep source files unchanged during a run. Run comparisons one after another on this host. Keep background builds and database tests out of the measurement interval.

The runner creates its own project, checks exact IDs, image IDs, labels, temporary storage and loopback ports, applies migrations and roles only there, and compiles the actual test and worker binaries. Workload logins remain separate from the isolated administrative fixture login. On cancellation it stops its own processes and checks the exact temporary targets before removal. Raw environment files are private and excluded from the artifact manifest. The browser cannot start faults or load raw host artifacts.

The qualified measurement build is identified by each run below. Later documentation and host-cancellation-check changes do not relabel those binaries. The phase completion record binds the separate logs and states that source distinction.

- `demo-20261015130819-c3dc94c1`: source manifest SHA-256 `e1fd9f1089c4f2c7b7f6edcbf62e7eef819a45213692dad6e232d2ed90eb7e51`. Its ignored bundle is `data/portfolio/demo-runs/demo-20261015130819-c3dc94c1`.
- `demo-20261015131550-4e98aa15`: source manifest SHA-256 `e1fd9f1089c4f2c7b7f6edcbf62e7eef819a45213692dad6e232d2ed90eb7e51`. Its ignored bundle is `data/portfolio/demo-runs/demo-20261015131550-4e98aa15`.
- `demo-20261015132320-5da99c95`: source manifest SHA-256 `e1fd9f1089c4f2c7b7f6edcbf62e7eef819a45213692dad6e232d2ed90eb7e51`. Its ignored bundle is `data/portfolio/demo-runs/demo-20261015132320-5da99c95`.

## Evidence and historical studies

See [R09 completion](../release-evidence/portfolio/r09/completion.json), [the independent financial model](financial-model.md), [the guided failure suite](failure-demonstration.md), and [the issue register](issues.md). Raw bundles stay in ignored local storage. Completion records retain their hashes.

The [older fixed-demand study](../release-evidence/unit-3-unit-4/performance-report.md) and [older API capacity report](../performance-baseline.md) remain historical evidence for their recorded candidates and workloads. They do not qualify this changed role and tracing implementation. Conversely, this component result does not replace an HTTP/BFF or multi-host release measurement. R13 and the final affected-feature repeat own those release checks.
