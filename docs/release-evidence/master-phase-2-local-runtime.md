# Master Phase 2 deterministic local runtime evidence

**Qualified implementation commit:** `bddc35cdc8f3eda1c57893884598688b4b50ad35`

**Captured:** 2026-08-28 (Asia/Calcutta)

**Decision:** Phase 2 exit gate passed; Phase 3 may begin

## Delivered operator contract

- `scripts/doctor-local.ps1` performs a read-only prerequisite and runtime diagnosis before an operator changes local state.
- Shared diagnostics distinguish Docker not installed, Docker Desktop stopped, engine permission failures, unsupported Compose versions, insufficient disk, occupied ports, missing environment state, and unavailable volumes.
- Every unavailable service reports the affected LedgerSync capabilities and an exact recovery action.
- Startup validates prerequisites before mutation, waits on health-based readiness, and preserves loopback-only host exposure.
- Log output includes timestamps, shutdown allows a 30-second graceful drain, and destructive reset requires confirmation while preserving the backup directory.
- Reset reports whether a validated backup and restore drill exists; it never represents absent recovery evidence as safe.
- Demo data is versioned and idempotent, and refuses to apply an older seed over persisted state created by a newer seed version.

## Exact-commit GitHub evidence

| Workflow/job | Result | Evidence |
|---|---|---|
| Contract validation | Passed | [run 33143312234](https://github.com/painjanevivek/LedgerSync/actions/runs/33143312234) |
| Production-path CI | Passed | [run 33143312221](https://github.com/painjanevivek/LedgerSync/actions/runs/33143312221) |
| Supply-chain and security gates | Passed | [run 33143312218](https://github.com/painjanevivek/LedgerSync/actions/runs/33143312218) |
| Quality gates | Passed | [run 33143312238](https://github.com/painjanevivek/LedgerSync/actions/runs/33143312238) |
| Local operator diagnostics and safety contracts | Passed | Doctor boundaries, dependency classification, service guidance, private bindings, graceful shutdown, seed compatibility, reset disclosure, and timestamped-log assertions |
| Web quality | Passed | Lint, 79 tests, optimized build, and JavaScript/font budgets |
| Browser quality | Passed | Responsive, accessibility, visual-regression, performance, and CLS checks |
| Live dependencies | Passed | PostgreSQL/Redis integration and dependency-fault suites |
| Real stack | Passed | BFF/API/PostgreSQL/Redis/worker startup, retry and lost-response recovery, seed rerun safety, service/Redis/PostgreSQL restart, migration and reconciliation, protected backup/isolated restore, dependency-fault recovery, non-root execution, and private bindings |
| Production containers | Passed | API, worker, and web images built from pinned bases |
| Containers/provenance | Passed | API, worker, and web high/critical scans; three SBOMs; three attestations |

## Local Windows corroboration

- The read-only doctor correctly identified Docker Desktop as installed while distinguishing its stopped engine from a missing installation or permission failure.
- The doctor reported 805.7 GiB of available disk, the Compose definition, and protected runtime environment state as passing checks.
- Compose, port-ownership, and volume checks were explicitly reported as blocked by the stopped engine, not misreported as healthy or empty.
- PowerShell parser validation and the local acceptance, API credential, database role, demo, recovery, runtime doctor, and Phase 10 preparation contract suites passed.
- `docker compose config -q` passed with isolated ephemeral configuration.
- Go command, internal, unit, and contract tests passed; all 79 web tests passed.

Docker Desktop's Linux engine was not started during the Windows corroboration run. Operational start, stop, restart, seed, backup, isolated restore, and fault recovery were instead proven by the exact-commit Linux real-stack job. This distinction is retained so local workstation state is never presented as runtime evidence it did not produce.

## Exit-gate decision

- A non-technical operator receives preflight diagnostics, affected-capability explanations, and actionable recovery guidance before local startup mutates state.
- Supported lifecycle commands cover start, diagnose, timestamped logs, graceful stop, restart, backup, restore, and confirmed reset without exposing infrastructure services publicly.
- Health gates and exact-commit real-stack evidence prove the runtime reaches readiness and survives the documented restart and dependency-failure scenarios.
- Reset and demo operations are guarded against silent recovery-evidence loss and incompatible seed downgrades.
- No quality, financial, security, accessibility, performance, or recovery threshold was weakened.

Phase 2 is complete. This evidence qualifies the repository-supported local runtime only; it does not claim that an arbitrary workstation with a stopped or misconfigured Docker engine is healthy, and it does not satisfy physical-device, managed-provider, legal, partner, or production gates.
