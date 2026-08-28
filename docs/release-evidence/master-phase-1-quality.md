# Master Phase 1 quality reconvergence evidence

**Qualified implementation commit:** `417bd0b5f08f0e00e8d2c74bcda331cf09cf6ec5`

**Captured:** 2026-08-28 (Asia/Calcutta)

**Decision:** Phase 1 exit gate passed; Phase 2 may begin

## Remediated failures

The previous responsive candidate `1fa7709` had two stop-ship failures:

- `TestEventEvidenceAuthorizationPaginationAndFirstClaimTruth` used a fixed application clock older than PostgreSQL's real `available_at`, making outbox claiming date-dependent.
- The pinned Alpine runtime carried OpenSSL `3.5.7-r0`, which the current vulnerability database identified as affected by high-severity `CVE-2026-14456`; Alpine supplied fixed `3.5.8-r0` packages.

The remediation derives the outbox test clock from persisted database evidence and upgrades runtime packages during API, worker, and web image construction. No financial, CLS, performance, coverage, or security threshold was loosened and no vulnerability suppression was added.

## Exact-commit GitHub evidence

| Workflow/job | Result | Evidence |
|---|---|---|
| Production-path CI | Passed | [run 33142255850](https://github.com/painjanevivek/LedgerSync/actions/runs/33142255850) |
| Production Go | Passed | Formatting, full vet, and race tests |
| Production web | Passed | Clean install, lint, and optimized build |
| Production containers | Passed | API, worker, and web images built from pinned bases |
| Supply-chain and security gates | Passed | [run 33142255836](https://github.com/painjanevivek/LedgerSync/actions/runs/33142255836) |
| Secrets/dependencies/IaC | Passed | Gitleaks, Go vulnerability scan, npm production audit, and configuration scan |
| Containers/provenance | Passed | API, worker, and web high/critical scans; three SBOMs; three attestations |
| Quality gates | Passed | [run 33142255853](https://github.com/painjanevivek/LedgerSync/actions/runs/33142255853) |
| Go quality | Passed | Formatting, vet, static analysis, race, exact-money fuzz, and critical-core coverage floor |
| Web quality | Passed | Lint, 79 tests, optimized build, and JavaScript/font budgets |
| Browser quality | Passed | Responsive, accessibility, visual-regression, performance, and CLS checks with the unchanged `0.1` ceiling |
| Live dependencies | Passed | PostgreSQL/Redis integration and fault suites, including the outbox regression |
| Real stack | Passed | BFF/API/PostgreSQL/Redis/worker startup, exact retry, lost response, seed safety, service/Redis/PostgreSQL restart, reconciliation, protected backup/isolated restore, dependency-fault recovery, non-root and private-binding checks |
| Release evidence | Passed | Commit-bound machine-readable release manifest generated and uploaded |

## Local corroboration

- Go formatting, vet, command/internal/unit/contract tests passed on Windows with Go 1.26.6.
- Web lint, 79 tests, Next.js production build, and frontend bundle budgets passed with Node.js 24.12.0 and npm 11.6.2.
- The production build emitted the documented operator/BFF route inventory without an unexpected route or compile failure.
- The working tree was clean after the remediation commit and matched `origin/main`.

Docker Desktop's local Linux engine did not start inside the bounded local readiness attempt. This did not produce a false local-ready claim: Linux container, dependency, real-stack, backup/restore, and fault evidence came from exact-commit GitHub runners. Deterministic local engine diagnosis is explicitly Phase 2 work.

## Exit-gate decision

- Required exact-commit workflows passed.
- CLS remained at or below the unchanged `0.1` budget through browser quality.
- Financial invariants, tenant authorization, idempotency, reconciliation, retry, recovery, and Redis-disposability scenarios passed.
- API, worker, and web images built and scanned with zero open high/critical result.
- README, master register, historical-plan authority, route/build output, and release evidence agree on the local-only boundary.

Phase 1 is complete. This evidence qualifies the local release candidate only; it does not satisfy physical-device, finance, AWS/Cognito, provider PITR, legal, penetration-test, partner, or production gates.
