# Phase 17 full qualification and release evidence

**Automated qualification:** `PASS`

**Production decision:** `NO-GO — MANUAL AND EXTERNAL GATES REMAIN`

**Qualification run:** `2026-09-01_5370c4c_windows-local_and_github-ubuntu`

**Qualified executable candidate:** `5370c4c18e5ceb93a345b655dd5819d51b8ef332`

This document is the documentation-only successor to the qualified executable candidate. It does not convert missing human or managed-environment evidence into a pass.

## Plain-language decision

The repository implementation is ready for controlled review: source, contracts, browser behavior, production builds, live PostgreSQL and Redis behavior, Docker topology, recovery, fault handling, secret scanning, vulnerability scanning, container scanning, SBOM generation and provenance all passed on one exact commit.

LedgerSync is **not approved for production deployment**. A human accessibility review has not been performed by named reviewers, an approved dependency-license policy is not present, browser observability cannot be enabled safely without privacy and retention decisions, and the managed identity, secrets, edge, backup and operations environment from Phase 19 does not exist in this repository. Engineering evidence is necessary but is not a substitute for product, security, accessibility or operations approval.

## Candidate and environment identity

| Item | Recorded value |
|---|---|
| Candidate commit | `5370c4c18e5ceb93a345b655dd5819d51b8ef332` |
| Candidate branch | `main` |
| Qualification date | 2026-09-01 UTC |
| Local operating system | Windows amd64 |
| CI operating system | GitHub-hosted Ubuntu 24.04 |
| Go | 1.26.6 |
| Node.js | 24.12.0 locally; repository engine floor `>=20.18.0` |
| npm | 11.6.2 locally |
| Playwright | 1.62.1 |
| Browser | Bundled Playwright Chromium 1.62.1 |
| Database/cache evidence | Disposable PostgreSQL 16 and Redis 7.4 containers in CI |
| Local Docker qualification | Not available because the workstation Docker daemon was not running |
| Docker replacement evidence | Exact-commit CI real-stack and container jobs passed; the local absence is not recorded as a pass |

The worktree contained one unrelated untracked `docs/academic/` directory supplied by the user. It was excluded from every commit in this qualification sequence.

## Exact-commit workflow record

| Workflow | Result | Evidence |
|---|---|---|
| Quality gates | `PASS` | https://github.com/painjanevivek/LedgerSync/actions/runs/33460264393 |
| Production-path CI | `PASS` | https://github.com/painjanevivek/LedgerSync/actions/runs/33460264397 |
| Supply-chain and security gates | `PASS` | https://github.com/painjanevivek/LedgerSync/actions/runs/33460264396 |
| Contract validation | `PASS` on the immediately preceding runtime candidate; the performance-only change did not touch contracts | https://github.com/painjanevivek/LedgerSync/actions/runs/33459470928 |

The quality workflow completed all six jobs: web quality, Go quality, browser quality, live dependencies, real stack and release evidence.

## Automated source and contract qualification

The following local commands passed after the final implementation changes, with equivalent Linux gates repeated in CI:

```text
go test ./cmd/... ./contracts/... ./internal/... ./tests/... -count=1
go vet ./cmd/... ./contracts/... ./internal/... ./tests/...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1 run
npm run lint
npm test
npm run build
npm run test:performance
npm run test:e2e:performance
npm run test:e2e
```

Recorded results:

- Go formatting, compilation, deterministic tests and vet passed.
- Pinned `golangci-lint` 2.13.1 reported `0 issues`.
- Linux race tests passed in CI.
- The exact-money fuzz boundary passed its required CI interval; a longer local run exceeded 282,000 executions without failure.
- Critical financial-core statement coverage was **68.1%**, above the required 60% floor.
- Local runtime doctor, Docker failure classification, service recovery guidance, loopback restrictions, graceful shutdown, seed safety and reset safety passed in CI.
- All **175/175** frontend unit, security, system-boundary and UI contract tests passed.
- The production Next.js build, ESLint, static asset budget and generated developer-contract convergence passed.
- OpenAPI validation passed on the nearest contract-bearing candidate, and the final change affected only performance evidence collection.

## Browser, responsive and accessibility automation

The complete browser workflow passed **206/206** Playwright checks on the local Windows baseline before the final backend-only fixes. The exact final commit passed the Linux browser-quality job with all applicable checks and four explicitly Windows-only comparisons skipped by platform design.

Automated coverage includes:

- authenticated operator journeys and direct-route authorization denial;
- funding, approvals, transfer, correction, investigation, reconciliation, operations, developer and recovery surfaces;
- failure, empty, unavailable, unknown-outcome and retry states;
- Chromium accessibility checks using Axe WCAG A/AA rules;
- keyboard-oriented interaction contracts and dialog behavior covered by browser automation;
- compact, mobile, tablet, desktop and ultrawide reflow;
- Windows and Linux platform-specific screenshot baselines;
- forced-colors, reduced-motion and semantic state contracts where automated coverage is reliable;
- progressive rendering for 100 bounded ledger rows and 25 bounded approval records without blocking navigation.

Automated tools do not prove screen-reader comprehension, physical-device usability or every focus/cognitive issue. Those remain manual gates below.

## Performance and bundle report

The production-mode performance suite passed **3/3** scenarios. The primary profile used a 390 × 844 viewport, 4× CPU throttling and constrained 4G network settings.

| Budget | Observed | Limit | Result |
|---|---:|---:|---|
| Initial browser requests | 32 | 32 | `PASS` |
| Initial API requests | 5 | 8 | `PASS` |
| Initial encoded transferred bytes | 257,620 | 3,000,000 | `PASS` |
| Largest Contentful Paint | 1,332 ms | 2,500 ms | `PASS` |
| Interaction to Next Paint | 32 ms | 200 ms | `PASS` |
| Cumulative Layout Shift | 0.00109 | 0.10 | `PASS` |
| Maximum long task | 96 ms | 250 ms | `PASS` |
| Long-task total | 96 ms | 1,500 ms | `PASS` |

The production static-asset report recorded:

| Asset measure | Observed | Limit | Result |
|---|---:|---:|---|
| JavaScript chunks | 41 | informational | `PASS` |
| Total JavaScript | 1,552,150 bytes | 2,000,000 bytes | `PASS` |
| Largest JavaScript chunk | 229,156 bytes | 350,000 bytes | `PASS` |
| Font files / total | 0 / 0 bytes | 320,000 bytes total | `PASS` |

The transferred-byte assertion was added during Phase 17 because the audit ceiling existed in prose but was not previously executable. It now appears in both console output and the attached Web Vitals JSON.

## Live dependencies, real stack and failure injection

The exact final commit passed live PostgreSQL/Redis integration and the complete Docker real-stack sequence.

| Real behavior | Result |
|---|---|
| Build and start real BFF, API, PostgreSQL, Redis and worker | `PASS` |
| Browser-facing same-key retry safety | `PASS` |
| Lost response followed by same-key replay after API restart | `PASS` |
| Repeated demo seed cannot rewind financial state | `PASS` |
| Stateless API/web and Redis restart recovery | `PASS` |
| PostgreSQL restart, repeatable migration and matched reconciliation | `PASS` |
| Digest-bound backup and isolated restore | `PASS` |
| Redis loss and rebuild from PostgreSQL | `PASS` |
| Redis unavailable while PostgreSQL remains authoritative | `PASS` |
| PostgreSQL unavailable returns sanitized 503 rather than false empty/success or misleading 401 | `PASS` |
| Dependency-order shutdown and full recovery | `PASS` |
| Financial fingerprint unchanged across safe fault drills | `PASS` |
| Non-root, read-only workloads and private service bindings | `PASS` |

The run found and closed five qualification defects before approval:

1. Linux visual baselines had drifted from reviewed Windows behavior and lacked six multi-capture states. Exactly 34 CI actual screenshots were reviewed and promoted without weakening tolerances.
2. Investigation authorization skipped PostgreSQL parameter positions for non-account sources. Domain-specific arguments now use contiguous typed placeholders.
3. Relationship CTEs relied on inferred literal-expression names and untyped unused parameters. All seven domains now share explicit result columns and a typed parameter envelope.
4. Real-stack tests used a retired caller-controlled `require_version` query. They now prove read-your-writes through the rotated signed session used by production.
5. Replay-guard database outages were collapsed into invalid credentials. Invalid/replayed assertions remain 401, while dependency outages fail closed with sanitized 503 responses across handlers.

## Security and supply-chain summary

The exact final commit passed:

- full-history redacted Gitleaks scanning;
- pinned `govulncheck` 1.7.0 scanning;
- production `npm audit --omit=dev --audit-level=high` with zero vulnerabilities;
- Trivy configuration/IaC scanning;
- immutable API, worker and web image builds;
- high/critical Trivy image gates for all three images;
- SPDX SBOM generation for all three images;
- GitHub artifact provenance attestations for each SBOM.

Local `govulncheck` reported **zero called vulnerabilities**. It also reported GO-2026-5158 in imported-but-unreachable OpenTelemetry 1.43.0 code, with 1.44.0 identified as the fixed line. This is scheduled dependency remediation, not a claim that an exploitable LedgerSync call path was found.

## Preserved artifacts

| Artifact | SHA-256 digest |
|---|---|
| `phase-0d-release-evidence-5370c4c18e5ceb93a345b655dd5819d51b8ef332` | `95e88605f0583481d82965d60d85117cc3dcea99c080980f5cfc1c912f51aa1e` |
| `operator-browser-evidence-5370c4c18e5ceb93a345b655dd5819d51b8ef332` | `dea905be23b7081c53df0ad95f1b46f132d816981ae52947ee6bfccc5c097b1a` |
| `critical-financial-core-coverage` | `9acb7076160358de475ac6ab80907a1d0ce1ee25ade1137f7eb87065e84dd1d6` |
| `ledgersync-api-sbom-5370c4c18e5ceb93a345b655dd5819d51b8ef332` | `9c1f221d47eb81abfa25b84f6d587c1ae007242d01f6a2bdc7368f69e03145ec` |
| `ledgersync-worker-sbom-5370c4c18e5ceb93a345b655dd5819d51b8ef332` | `ad923ee9b47fff6bee27a7a2bde44e05393f6e18fa7f9272716c1dfb6899088c` |
| `ledgersync-web-sbom-5370c4c18e5ceb93a345b655dd5819d51b8ef332` | `b341bea582b0227209339620b64f812aee0c3ce21d4cdde260e6d9040020987b` |
| `gitleaks-results.sarif` | `6f390f206f7f00b53441165cc05515504ac5aa693228140ed678be2c7b29fa7c` |

GitHub Actions artifacts are retained according to repository/platform retention settings. Expiry must be checked before an audit; important evidence should be copied into the approved long-term evidence store by its owner.

## Manual accessibility checklist — required before release

No named human reviewer was available during this implementation run. Every item below is therefore `PENDING`, even where automated coverage exists.

| Manual check | Required evidence | Owner | Status |
|---|---|---|---|
| Keyboard-only traversal of every critical workflow | Dated notes for overview, accounts, funding, approvals, transfers, corrections, investigation, reconciliation, operations, developer and recovery | Accessibility reviewer — name required | `PENDING` |
| Focus visibility and logical order | No hidden focus; order follows visual and reading order | Accessibility reviewer — name required | `PENDING` |
| Dialog trap, Escape and focus restoration | Funding, correction, replay and destructive confirmation dialogs | Accessibility reviewer — name required | `PENDING` |
| 200% zoom and narrow viewport | No lost information, overlap, clipping or unreachable action | Accessibility reviewer — name required | `PENDING` |
| Windows screen reader/browser | NVDA with supported Chrome or Edge; record version and findings | Accessibility reviewer — name required | `PENDING` |
| Second platform screen reader/browser | VoiceOver/Safari or approved equivalent; record version and findings | Accessibility reviewer — name required | `PENDING` |
| Dynamic status announcements | Loading, success, error, unavailable and unknown outcomes are useful without repetition | Accessibility reviewer — name required | `PENDING` |
| Tables and reading order | Headers, row context and responsive presentation remain understandable | Accessibility reviewer — name required | `PENDING` |
| Chart alternatives | Every visual trend has equivalent text/table evidence | Accessibility reviewer — name required | `PENDING` |
| Reduced motion and forced colors | Critical state and focus remain visible and understandable | Accessibility reviewer — name required | `PENDING` |
| Physical phone/tablet smoke | Touch targets, virtual keyboard, rotation and safe-area behavior | Product/accessibility reviewer — name required | `PENDING` |

Release criterion: named reviewers must sign and date this checklist, attach issue references for findings, and leave no unresolved critical accessibility issue.

## Known-risk and external-gate register

| ID | Classification | Required action | Accountable owner | Due/expiry | Status |
|---|---|---|---|---|---|
| R17-01 | Release blocker | Complete and sign the manual accessibility checklist on two platform combinations | Accessibility owner — unassigned | Before any production release | `OPEN` |
| R17-02 | Release blocker | Define an approved dependency-license allow/deny policy and run a candidate-bound license review | Legal/security owner — unassigned | Before any external distribution | `OPEN` |
| R17-03 | Release blocker | Approve browser telemetry schema, allowlist/redaction, consent, retention/deletion, vendor/collector and incident owner | Privacy, security and SRE — unassigned | Before Phase 15 telemetry or production release | `OPEN` |
| R17-04 | Release blocker | Provision and prove managed OIDC, secrets/KMS, TLS/WAF, network isolation, backups/PITR, alerting and operational runbooks | Platform/SRE and security — unassigned | Phase 19, before production | `OPEN` |
| R17-05 | Scheduled remediation | Upgrade OpenTelemetry 1.43.0 to a compatible fixed line at or above 1.44.0 and rerun all telemetry/security gates | Engineering dependency owner — unassigned | Target 2026-09-08 | `OPEN` |
| R17-06 | Evidence-retention risk | Copy expiring GitHub artifacts into an approved immutable evidence store and verify digests | Release/operations owner — unassigned | Before platform artifact expiry | `OPEN` |
| R17-07 | Product decision | Decide whether Phase 18 public trust site is required, approve claims/audiences and create its separate deployment boundary | Product/legal/security — unassigned | Before public launch | `OPEN` |

No risk above is silently accepted. An owner with authority must either close it with evidence or record an explicit time-bounded acceptance.

## Phase disposition beyond executable qualification

- **Phase 15:** browser observability remains gated. Adding a vendor or collector without privacy, consent, retention and incident ownership would violate the plan.
- **Phase 16:** completed; the retired legacy application slice was removed in dedicated commit `0046f0a` and is recoverable through Git.
- **Phase 17 automated work:** completed and reproducible from `5370c4c`.
- **Phase 17 manual/external work:** blocked on named human and organizational owners; production remains no-go.
- **Phase 18:** intentionally planned as a separate public website/trust surface. It was not invented inside the authenticated runtime because audience, approved claims, legal content and publishing ownership are external product decisions.
- **Phase 19:** cannot be implemented from repository code alone. Managed identity, key custody, edge controls, production recovery and operational approval require selected infrastructure and accountable owners.
- **Phase 20:** future add-ons remain evidence-led. No global store, search cluster, AI SDK, vector database, wholesale component library or telemetry vendor was added without a measured need and approved owner.

## Required sign-off

| Gate | Signer | Decision | Date |
|---|---|---|---|
| Engineering automated qualification | Evidence in exact-commit CI | `PASS` | 2026-09-01 |
| Engineering release owner | Name required | `PENDING` | — |
| Product owner | Name required | `PENDING` | — |
| Accessibility reviewer(s) | Names required | `PENDING` | — |
| Security/privacy reviewer | Name required | `PENDING` | — |
| Operations/SRE owner | Name required | `PENDING` | — |
| Legal/license reviewer | Name required | `PENDING` | — |

## Final decision

**Controlled engineering candidate:** `GO` for continued review, manual validation and managed-environment preparation.

**Production deployment:** `NO-GO` until R17-01 through R17-04 are closed, R17-05 and R17-06 have accountable dispositions, and engineering, product, accessibility, security/privacy, operations and legal sign the exact candidate evidence.

