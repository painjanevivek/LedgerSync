# Audit repair progress — 17 October 2026

The starting main commit is `9b2dfaf0bc0d7c91d345e28541b02a7afd7d72c8`. The [audit](audits/r00-r17-main-2026-10-17.md) remains a dated assessment of that commit. New source changes are uncommitted. Do not assign its earlier hosted CI result or image qualification to these changes.

Docker, normal services and deployments remain stopped. The normal schema64 database, original images, volumes, backups, stashes and recovery references are retained. No financial command, seed or migration is part of this repair pass. Native tests use synthetic fixtures and owned temporary processes.

## Phase checkpoints

| Phase / audit item | Source repair | Verification now | Acceptance still required |
|---|---|---|---|
| R06 / A01 | Process-local pool wait counters; fixed role/state labels; tenant-policy lock-call duration; dashboard panels; executable alert tests and runbook check; opaque webhook trace header; bounded trace-capture structure verifier | Affected Go tests passed with actual local TLS/HTTP fixtures. Native promtool 3.15.0 syntax and three rule cases passed. Trace verifier tests passed. | Full real command capture through BFF/API/database/worker/receiver; actual collector regression with changed source. Rule evaluation does not prove an external notification route. |
| R07 / A02 | Read-only preview; explicit mapping of all 13 required matrix rows to 14 fixed scenarios; after-commit HTTP connection abort; surviving owned HTTP API; restricted-role normal publication; preserved legacy summaries and current matrix version | Preview runs with child execution and file writes forbidden. Host safety tests passed. Integration tests compile. | Execute the new 14-scenario path against verified disposable PostgreSQL/Redis. Do not call a compiled test a runtime pass. |
| R08 / A03 | Independent lifecycle/approval workflow; deterministic 64-step database histories; bounded serial-order search; competing debit/bidirectional histories; shared observation comparators; seeded harness-adapter defect controls | Model unit tests passed. New integration tests compile. Original replay seeds remain compatible. | Run generated workflow/history database comparisons. The mutation controls alter adapter observations; they do not claim that mutated production SQL was executed. |
| R09 / A04 | Separate mixed/independent/hot/multi-tenant profiles; timed closed clients with operation bound; PID/executable/creation-bound worker CPU/RSS; resource comparison plan and complete-cohort checks | Profile/options, native owned-process sampler and comparison tests passed. | New sustained measurements, complete resource comparison and HTTP/BFF path. The comparison plan has 27 serial cases and no execution authority. Existing native component runners do not execute that container HTTP/BFF matrix. |
| R05/R14 / A05 | Fresh-person protocol, explicit success conditions, private record template and checker that cannot grant human approval; owner nominated Lord Master | Record/template tests are part of the offline host suite. | Lord Master's observations and independence attestation. Runtime tasks remain not run while Docker is stopped. |
| R13 / A06 | Dependency advisory rechecked; one native loopback frontend; timing diagnostics; report environment hygiene; unchanged budgets | Five HIGH development-only findings remain. Latest braces is 3.0.3. Native Windows LCP repeats failed at 2952, 2848 and 2860 ms against 2500 ms. Two companion performance checks passed. The failed chunk-size experiment was withdrawn. | P-062 has no upstream patched version. P-106 and complete changed-source runtime/image qualification remain open. |
| R14 / A07 | Current source/qualification distinction and superseded status pointers | Documentation checks will run after status edits. | Keep current status separate from immutable old completion records. |
| R17 / A08 | Normal-runtime cutover remains excluded; image source and managed inputs retain separate identity | Deployment controls remain unchanged. | No current-image, managed identity, recovery, publication or deployment pass is claimed for these source changes. |

## Verified evidence and retained failures

Private logs are under `data/portfolio/audit-remediation-20261017/`. The before manifest records 1,900 existing source paths and the original index digest. The external plan before-copy remains there too. No original audit or phase completion artifact is rewritten.

- `r06-red.txt` and `r08-red.txt` record the initial missing implementation failures.
- `r06-observability.txt` records the sandbox's blocked TLS loopback connection. The permitted repeat passed; the test and thresholds were unchanged.
- `r07-host-tests.txt` and `host-suite-sandbox.txt` retain the blocked temporary symlink result. The final permitted host suite, `host-suite-complete.txt`, passed 61 tests with no skips.
- `go-offline-suite.txt` records the full offline Go package suite. `go-vet-permitted.txt` records static checks. The pure profile-distribution test passed; integration compilation is separate from database execution.
- `r06-example-verified.txt` records all six TypeScript example tests. `web-types.txt`, `example-types.txt` and `web-lint.txt` record passing type and lint checks.
- `golangci-lint.txt` records zero issues with the existing pinned v2.13.1 checker across production and test roots. `web-unit.txt` records 254 passing existing tests. `browser-config-privacy.txt` records the additional synthetic-credential regression; final type and lint checks pass.
- `web-build-restored.txt` and `web-assets-restored.txt` record the restored production build and passing asset gate: 1785515 JavaScript bytes, largest chunk 229820 bytes. The failed experiment remains in `web-assets-chunking-failed.txt`.
- `r06-alerts-test.txt` and `r06-alerts-wrapper.txt` record actual native rule evaluation, including firing, pending delay, recovery and negative cases.
- `r09-native-resources.txt` measures an owned test child. It verifies the sampler, not application throughput or worker peak memory under load.
- `npm-audit-current.json` and `braces-current-version.json` retain the actual current dependency response. No package downgrade, force fix or advisory suppression was made.
- `windows-performance-repeat.txt`, `windows-performance-diagnostic.txt` and `windows-performance-chunking.txt` retain failed Windows measurements. Their JSON and browser artifacts have separate retained copies. The setup failure has its own record.

The chunk experiment reduced initial document/script/style/API requests from 30 to 23 and transferred bytes from 285020 to 277027 in the same mocked journey. However, total emitted JavaScript increased to 2031822 bytes, above the unchanged 2000000-byte limit. The experiment was withdrawn. Original chunk settings are restored. This finite experiment is not a throughput or statistical claim. Its timing capture measured observer start at 2182 ms and LCP at 2860 ms. This locates a delay before page scripts begin; it does not establish its cause. The LCP budget, four-times CPU throttle, constrained network, session delay and all assertions remain unchanged.

The audit frontend binds only `127.0.0.1:3117`. API calls are synthetic browser mocks; the private API address is an unused loopback port. Each Playwright-owned server exits after its run. Private JSON reports must not be published: old reports included the parent's environment because the initial configuration copied it. P-114 removes that explicit environment copy from the audit, default and pinned-Linux configurations. Server environment inheritance is unchanged.

The [offline verification record](../release-evidence/portfolio/audit-remediation-20261017.json) lists scoped results, evidence hashes and remaining gates. The external plan is now v1.29. All 18 original phase sections remain. Before-copies of v1.27 and v1.28 remain private.

The official [Prometheus rule-test format](https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/) supports native evaluation without a server. The official [braces advisory](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm) still lists no patched version at this check. Keep the existing runtime exclusion and repository-controlled lint-pattern limit; do not claim that all dependencies are clean.

## Safe next verification

Continue all source, unit, type, lint, documentation and native browser checks that do not start Docker. Use the [human review protocol](human-acceptance.md) to collect real acceptance observations. Keep runtime-dependent rows open. Before a later runtime run, freeze a new source manifest and verify exact disposable target exclusions. Repeat only affected checks and required integrated gates. Rebuild and rescan images only after that later runtime decision.

No commit, push, branch reset, data cleanup, deployment or normal-runtime upgrade was done in this repair pass. A passing offline check does not grant a new runtime or deployment approval.
