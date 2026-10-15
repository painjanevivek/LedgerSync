# Portfolio implementation packets

Owner: engineering delivery. Updated: 15 October 2026. The external portfolio plan remains the detailed product/design authority. This register makes its phases executable. The owner requested all R phases. See the latest completion entry and progress table for verified phases; earlier checkpoints remain as history. R15 uses a documented CSV format with synthetic samples. R16 uses the bounded full-capture reservation model in the plan. R17 prepares a production deployment package and keeps all execution local.

## Rules for every packet

Use the R00 manifest or a later recorded candidate as input. Read the named files and their callers before editing. Add a failing behaviour test before changing a financial boundary. Preserve stable public contract names, exact money, unknown outcomes, and all user data. Do not rewrite applied migrations. Use new additive migrations with collision-free versions.

Each table row states a result, affected files, verification, and repair. A row is complete only when its named tests pass, source identity and results are recorded, and issues have a disposition. A role is an ownership function, not a request to start a separate agent. Dependencies within a phase follow row order unless a row states otherwise. Existing callers can remain on a compatible path until the replacement passes.

For documentation/tooling, repair means revert only that packet's added changes from its recorded before-copy. For database work, repair means stop the candidate in the isolated stack, preserve failure evidence, and apply a reviewed forward fix. Do not restore broad grants or rewind the user's normal database to make a test pass. Never use a repository reset as rollback.

## R00 — Preserve and reconcile

Entry: authorised portfolio objective; read local instructions. Owner: delivery/platform.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R00.01 | Preserve the exact dirty workspace and index | Source-before and tracked R00 manifest | Compare original hashes and index at exit; stop on unexplained change. |
| R00.02 | Restore access to existing services and protect records | Existing runtime inventory, private backup bundle | Doctor/status, backup digest, normal financial fingerprints; retain original volumes/images. |
| R00.03 | Know what already exists | Module inventory, source inventory, reconciled status | Cover all packages/routes/optional areas; mark lexical/index limits. |
| R00.04 | Record a reproducible starting result | Go/UI/browser baseline logs, issues, progress | Distinguish source from existing images; preserve failures and historical results. |

Exit: all four packets verified with the external-index method deviation recorded. No normal financial mutation required.

## R01 — Boundaries and controls

Entry: R00. Owner: architecture/delivery.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R01.01 | Understand ownership, trust, outcomes, and navigation | Six ADRs; boundary walkthroughs | Walk transfer, correction, read, retry, export; record existing exceptions. |
| R01.02 | Prevent important dependency regressions | `tests/architecture/boundaries_test.go` | Positive/negative policy cases and real import scan; narrow a rule only with an ADR. |
| R01.03 | Use a versioned demonstration result | `contracts/demo/`, `scripts/portfolio/demo-contract*` | Reject false passes, bad targets, unsafe paths, missing reasons, oversized/invalid input; no runtime claim from fixture. |
| R01.04 | Run controls in normal delivery | Existing `.github/workflows/quality.yml` | Extend current Go and web jobs; run new local checks; inspect diff and syntax. Hosted CI executes after a later push. |
| R01.05 | Give every later phase a small starting packet | This register, progress, external plan update | All R00–R17 represented; preserve original plan context and open gates. |

Exit: checks pass and all packet records exist. No UI, financial schema, role, or capability change is part of R01.

## R02 — Trusted tenant database context

Entry: R01. Owner: database/security. Use a disposable database and actual workload logins.

Completion: R02.01–R02.07 are locally verified. Migrations 000044–000046 and actual workload tests cover caller adoption, forced policies, workers, role inheritance, and fresh/populated upgrades. See [adoption record](../security/tenant-context-adoption.md), [cutover procedure](../security/tenant-context-cutover.md), and [completion evidence](../release-evidence/portfolio/r02/completion.json). Normal runtime deployment and R03 write revocation remain separate gates.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R02.01 | Know every database access path and bypass | `deploy/postgres/roles.sql`, migrations, DB adapters, capability matrix | Inventory effective membership, owners, BYPASSRLS, views/functions, global queues, maintenance/backup users; reconcile live isolated roles. Repair inventory before policy changes. |
| R02.02 | Set tenant/actor once on the query transaction | New narrow helper beside `db/postgres.go`; identity adapter callers | Missing, malformed and forged input tests; context set on same connection; cancellation/rollback/pool reuse tests. Keep existing read contract while helper is tested. |
| R02.03 | Protect accounts, history, and financial records | Additive policy migrations; account/balance/history repositories | Real login tests for tenant A/B, direct IDs, joins, pagination, missing context, table owner behaviour; no cross-tenant rows/counts. Repair isolated policy before adoption. |
| R02.04 | Protect operational and saved evidence | Investigation, approvals, corrections, delivery, recovery/export adapters | Cross-tenant filters/cursors/exports and old record links; no existence leaks. Preserve permitted same-tenant results. |
| R02.05 | Give workers and maintenance narrow access | Worker/scheduler/reconciliation/retention adapters; roles | Narrow queue claim, scoped item processing, no general bypass; test hostile role inheritance and empty context. Stop adoption if a worker needs broad access. |
| R02.06 | Prove context does not leak under reuse | Integration/capability tests and safe CI disposable setup | Concurrent mixed-tenant calls, commit/rollback/cancel, prepared/query reuse, row writes; capture capability report. No tests against normal data. |
| R02.07 | Record the actual security claim | ADR-002, matrix, migrations runbook, progress | State trusted-workload limitation of mutable tenant settings; review upgrade compatibility and failures. Exit only after all scoped accesses pass. |

## R03 — Controlled financial writes

Entry: R02. Owner: financial/database. Existing controlled transfer functions are inputs, not missing features.

Completion: R03.01–R03.08 are locally verified. See [completion evidence](../release-evidence/portfolio/r03/completion.json) and the [cutover procedure](../security/controlled-write-cutover.md). Normal runtime deployment remains separate.

Historical checkpoint through 000054: account/provisioning, funding/correction request and decision, audit append, and worker completion callers are converted. The complete integration/fault suites and Go checks pass. This is partial acceptance. Recovery commands, reconciliation/maintenance authority review, additional hostile-input tests and final capability evidence remain. See [checkpoint](../release-evidence/portfolio/r03/checkpoint-000054.json).

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R03.01 | Map every writer before revoking grants | DB adapters, SQL functions, `roles.sql`, writer matrix | List direct INSERT/UPDATE/DELETE, defaults/sequences, worker/maintenance routes, error/audit paths. Mark each caller converted or pending. |
| R03.02 | Control account/provisioning commands | Account command/provisioning adapters, additive functions | Actor/tenant policy, idempotency, duplicate names/references, inactive accounts, audit atomicity; normal lifecycle regression. Retain old compatible caller until verified. |
| R03.03 | Qualify and tighten transfer commands | Existing controlled transfer functions/repository | Balanced journal, exact overflow/currency, deterministic locks, replay/conflict, concurrent funds, denial audit, lost response/request-reference recovery. Preserve existing function compatibility or version it. |
| R03.04 | Control funding and approval | Funding/approval repositories and SQL | Request/approve/reject/expire, independent approver, idempotency, policy snapshot, exact postings and audit; concurrent decision test. Forward-fix isolated failures. |
| R03.05 | Control compensation | Correction repository and functions | Original immutable; at most one permitted compensation; separate approver, expired approval, replay, concurrent decisions, linked records. Never delete original evidence. |
| R03.06 | Control operational writers | Outbox/delivery/verification/replay/audit adapters | Fenced lease/ack/reschedule, authorised replay, required denial evidence, durable outcome; no new financial posting during delivery repair. |
| R03.07 | Remove obsolete privileges after caller conversion | New migration/role script changes, capability tests | Deny direct protected DML, TRUNCATE, unsafe function execute, inheritance/default grant bypass; fixed search path and non-login function owners. Reject incompatible old binaries. |
| R03.08 | Qualify upgrade and publish boundary evidence | Isolated migration tests, database capability matrix | Upgrade populated old fixture and fresh install; full affected integration/fault paths. Record remaining exceptions; none may silently allow arbitrary financial writes. |

## R04 — Refactor by responsibility

Entry: R03 for affected financial paths. Owner: backend/frontend.

Completion: R04.01–R04.04 are locally verified for the selected worker, HTTP adapter and correction workspace hotspots. See [module map](module-map.md) and [completion evidence](../release-evidence/portfolio/r04/completion.json). The original identifier conversion exception remains explicit.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R04.01 | Select one high-cost responsibility | Inventory hotspots; import/caller maps | Distinguish view, orchestration, validation, and adapter duties; justify each split. No line-count-only move. |
| R04.02 | Separate backend adapters and transactions | Funding/correction/operations repositories, dispatchers, bootstrap | Characterisation tests plus architecture guard; retain one financial transaction owner. Revert a mechanical move if coupling rises. |
| R04.03 | Separate console state from presentation | Transfer/correction views, console shell, feature hooks | Existing state/command/browser tests; loading, denial, empty, unavailable, unknown retained. No route rename. |
| R04.04 | Remove proven duplication | Shared primitives only where behaviour matches | Compare both former callers; verify imports/public exports; update ADR exceptions. Keep local code if abstraction adds branches. |

## R05 — Console and first-run journey

Entry: R04 relevant boundaries; ADR-004. Owner: product/frontend.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R05.01 | Group routes without losing links | Console shell/navigation and compatibility map | Deep links, back/forward, filters, sort, role/feature visibility; preserve existing URLs. |
| R05.02 | Complete one clear first-run path | Overview, guidance, account/funding/transfer flows | Empty-state onboarding, required decisions, deterministic first transfer; no implied external funding/settlement. |
| R05.03 | Make high-risk outcomes clear | Transfer, approval, correction, recovery screens | Exact values, independent approval, unknown recovery, keyboard/focus/error states; use safe test fixtures. |
| R05.04 | Consolidate explanations and visual primitives | Guide, disclosure components, feature styles | Responsive and automated accessibility/visual tests; keep physical-device review unclaimed until done. Restore specific changed styles if regression appears. |

## R06 — Tracing and diagnostics

Entry: R04 stable owners. Owner: observability.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R06.01 | Follow one request across BFF/API/database | Existing observability and BFF adapters | Trusted propagation, allowlisted attributes, no authority from trace headers; negative secret/payload tests. |
| R06.02 | Follow delayed delivery attempts | Outbox/webhook/scheduler telemetry | Stable event identity, new attempt spans, explicit links; duplicate/retry/lease-loss traces. |
| R06.03 | Open permitted diagnostics from evidence | Developer/operations views, private collector config | Authorised links, missing/historical trace states, bounded export failure; ledger command remains available if collector fails. |
| R06.04 | Measure overhead | Trace test report/config identity | Bounded queues, shutdown, sampling, performance comparison; disable only added export path if it harms commands. |

## R07 — Guided failure demonstration

Entry: R03/R06; isolated target guard. Owner: systems/evidence.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R07.01 | Build a host-only, bounded runner | `scripts/portfolio/` and existing acceptance helpers | Reject normal project, foreign volumes/labels, unbounded input, arbitrary shell/fault targets. Validate before each fault. |
| R07.02 | Demonstrate lost response and duplicate delivery | Existing isolated fault/system fixtures | Same-key stored outcome, one financial effect, duplicate event handling; capture before/after invariant checks. |
| R07.03 | Demonstrate worker/cache/process failures | Existing lease/projection/scheduler/study tooling | Stale claim denied, monotonic versions, recovery within stated bound; single-host limitations recorded. |
| R07.04 | Produce verifiable artifacts | R01 schema, actual runner output, Systems Lab read view | Verify hashes, actual image/source/migration/config identity, scenario termination, cancellation and cleanup; show failed/incomplete honestly. Preserve bundle before removing isolated targets. |

## R08 — Stateful correctness verification

Entry: R03; use R07 fixtures where useful. Owner: financial verification.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R08.01 | Define an independent small financial model | New focused model tests under existing test layout | Model exact integer balances and journal conservation; avoid calling production calculation code as the oracle. |
| R08.02 | Generate bounded operation sequences | Property/stateful tests and seed records | Transfer/replay/conflict/funding/correction/failure sequences; record seed and shrink/minimise failure. |
| R08.03 | Compare model with actual isolated database | Integration harness | Concurrent schedules, unknown results, retries, retained version evidence; no skipped dependency reported as pass. |
| R08.04 | Retain every discovered counterexample | Regression fixtures and report | Reproduce smallest case before repair; affected tests plus model rerun after fix. |

## R09 — Capacity and measured improvements

Entry: correctness gates R03/R08. Owner: performance/database.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R09.01 | Define reproducible workload and limits | Existing benchmark/study harness; bounded config | Record hardware, topology, fixture, source/images, warmup, duration, concurrency, pool totals. |
| R09.02 | Measure representative and skewed traffic | Benchmark scripts/reports | Latency distributions, throughput, errors, saturation, queue lag, correctness; separate cold/warm and normal/hot-account results. |
| R09.03 | Fix one demonstrated bottleneck | Only measured adapter/query/config path | Before/after same workload; no weakened isolation, exactness, or audit. Revert change if improvement is not repeatable. |
| R09.04 | Publish limits honestly | Capacity note and artifact | State single-host scope and tested limits, not a production SLA; retain raw bounded measurements. |

## R10 — Backup, restore, migrations

Entry: R03; final repeat after all candidate schema changes. Owner: recovery/platform.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R10.01 | Verify backup bundle and retention safety | Existing backup scripts/metadata tests | Digest, completeness, permissions, failed/partial backup, no overwrite of good backup. |
| R10.02 | Restore into a verified isolated target | Existing restore drill | Reconcile accounts/journal/postings/projections and replay outcome; mismatch fails the drill. Never overwrite normal data. |
| R10.03 | Qualify schema upgrades | Fresh and populated migration fixtures | Versions, owners/policies/functions/default grants, interruption/retry, compatibility; use forward repair. |
| R10.04 | Re-run against final candidate | Recovery report and console evidence | Record actual recovery time/data boundary; reject stale or mismatched evidence. Keep managed recovery claim conditional. |

## R11 — Optional surfaces

Entry: R00 caller inventory, R05 navigation, R07 lab boundary. Owner: product/platform.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R11.01 | Define supported capability combinations | Existing config/bootstrap/BFF gates | Core; core+lab; core+explicit collaboration. Reuse current flags; direct calls fail closed when disabled. |
| R11.02 | Isolate simulation and study entry points | Payout simulation, study proxy, scheduler, commands/docs | No arbitrary browser fault execution; no external payment claim; default core operation intact. |
| R11.03 | Preserve historical optional records | Room/workspace routes, migrations, contracts/generated clients | Old links and retained evidence remain meaningful; no deletion without separate retention decision. |
| R11.04 | Qualify the supported matrix | Config/API/browser/contract tests | Enabled and disabled API routes, scopes, menus, jobs, environment failure; restore a compatible wrapper if an old caller breaks. |

## R12 — Independent developer integration

Entry: stable R03 contracts, R06 diagnostics. Owner: developer experience.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R12.01 | Choose one separate sample client | Existing OpenAPI/generated catalogs and a small example | Uses public API only; record supported language/tool versions; no internal import shortcut. |
| R12.02 | Complete useful workflow | Provision/account/read/transfer and webhook example | Exact values, scope denial, same-key replay/conflict, unknown recovery, endpoint verification and deduplication. |
| R12.03 | Reproduce from documented setup | Developer guide and quickstart tests | Follow steps from fresh isolated credentials/fixture; secrets kept out of sample; broken step blocks completion. |

## R13 — Integrated local qualification

Entry: R00–R12 complete for selected capability set. Owner: release verification.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R13.01 | Freeze an identified candidate | Source/image/migration/contract manifest | Dirty source digest if applicable; build identified images; no mixed-run pass aggregation. |
| R13.02 | Run existing complete gates plus new checks | Current CI, integration/fault/system/browser tests | Race/static/contract/architecture, tenant-role matrix, failure/model/capacity/recovery; record skips as gaps. |
| R13.03 | Review security, accessibility, compatibility | Issue register and evidence matrix | Enabled capability authority, redaction, route links, keyboard/responsive behaviour; human reviews stay explicit. |
| R13.04 | Repair then qualify affected candidate | Small fixes and new manifest | Rerun affected and required integrated gates; stop release on unresolved material failure. |

## R14 — Portfolio case study and handover

Entry: R13. Owner: product/documentation.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R14.01 | Explain problem, choices, and tradeoffs | README/case study/architecture diagrams | Link each claim to actual evidence; explain PostgreSQL authority and retry/delivery limits. |
| R14.02 | Give a short reproducible demonstration | Demo guide and safe sample artifacts | A new reader can run core and optional lab, find failure evidence, and stop safely. |
| R14.03 | Hand over maintenance duties | Operations guide and remaining issue list | Setup, upgrade, backup/restore, dependency updates, optional scope; no production claim from portfolio finish. |

## R15 — Selected external statement reconciliation

Entry: selected by the owner; R13-quality database/authority boundaries. Owner: reconciliation. Use one documented CSV format and synthetic samples. A bank connection is outside this phase.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R15.01 | Define one documented CSV source and mapping | Import contract/synthetic fixture | Currency, account mapping, timestamps, synthetic provenance, format limits, privacy; reject ambiguous input. |
| R15.02 | Import without touching ledger truth | New bounded import adapter/staging records | Duplicate file/row, malformed/partial/cancelled input, checksum and provenance. Preserve original source under defined retention. |
| R15.03 | Match deterministically and review exceptions | Reconciliation use case/UI | Versioned rules, repeatable matches, explicit unmatched/ambiguous cases; never auto-adjust a ledger. |
| R15.04 | Qualify as an enabled capability | Contract/security/replay/performance tests | Re-run integrated gates before presenting it as complete; disable new entry point if incomplete. |

## R16 — Selected holds and reservations

Entry: selected by the owner; R03/R08. Owner: financial domain. Use same-tenant, same-currency reservations with full capture or full release. Define all balance meanings before implementation. Exclude partial capture and external settlement.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R16.01 | Specify posted, held, and available values | Domain/contract design and worked examples | Exact equations, currency, expiry and concurrent command rules; no UI-first implementation. |
| R16.02 | Add controlled reserve/capture/release/expire | Additive tables/functions and application interfaces | Idempotency, partial rules if selected, no negative available value, fenced expiry; historical postings immutable. |
| R16.03 | Extend model and user evidence | R08 model, UI, exports, reconciliation | Interleavings, expiry/retry, approval and recovery; forward-repair data and repeat integrated qualification. |

## R17 — Local production deployment package

Entry: owner selected preparation only, with local execution; R13. Owner: platform/security. Cloud account, region, budget, domain and operator values remain explicit deployment inputs. They do not block local package preparation.

| Packet | User result and work | Files/output | Verification and repair |
|---|---|---|---|
| R17.01 | Define deployment inputs and operating duties | Environment template, threat/deployment plan and owner checklist | Required values and cost limits are explicit; no invented account, domain, budget or human owner. |
| R17.02 | Prepare infrastructure and release files | Container/config/secrets/deployment definitions | Least privilege, network isolation, migration safety, health checks and compatible rollback; secret-free templates. |
| R17.03 | Validate the package locally | Configuration checks and isolated local deployment rehearsal | Verify startup, failure, shutdown and upgrade ordering; record provider-dependent checks as pending. |
| R17.04 | Hand over the reviewed package | Release manifest, runbooks, monitoring and managed qualification checklist | All local gates pass; deployment inputs and later go/no-go are clear. No cloud execution or public release. |

Managed identity, provider restore/failover, external incident ownership and production approval remain future deployment gates. A complete local package does not satisfy those gates.

## Immediate next packet

Continue R02 with actor-aware legacy reads and exports, remaining scoped writers and narrow worker access. Add permanent row policies only after these callers pass real-role tests. Keep the normal local stack and R00 backup available. Do not start R03 privilege revocation before its callers have verified replacements.


## R05 completion — 15 October 2026

Navigation now groups health, events and recovery under Operations. Work and Investigate retain their routes and permissions. Developer retains its reference workspace. Help stays in the existing Guide utility to avoid duplicate links. The Overview requires a destination account before suggesting a first transfer. The Guide explains two accounts, independent review, posting, exact money, unknown outcomes and reconciliation limits. Existing visual primitives and high-risk controls remain.

Verification: 214 frontend tests, TypeScript, targeted ESLint, a production build and 56 browser tests passed. Browser checks cover role visibility, denied direct routes, back/forward query preservation, first-run actions, funding-post order, transfer review, historical/unavailable/unknown evidence, sign-out, keyboard focus, reduced motion, forced colors, zoom-equivalent reflow and automated accessibility. API responses were mocked and the test server used a disabled backend address.

Preservation: All phase-entry source files remain. The Git index and HEAD are unchanged. Normal financial fingerprints match R04, and the five preserved services are healthy. No normal migration, financial command, image rebuild or cloud action ran.

Issues and limits: No physical-device or human assistive-technology acceptance claim. The existing Guide utility represents Help; duplicate sidebar links were not added. The normal images are preserved prior images; candidate image qualification remains R13. R06-R17 remain selected implementation work. See [completion evidence](../release-evidence/portfolio/r05/completion.json). R06 is next.


## R06 completion — 15 October 2026

The existing telemetry stack now links a bounded BFF private-context span to API/database work. Transaction-local diagnostic context becomes optional immutable outbox origin evidence. Redis publication, projection and webhook delivery attempts have separate linked spans. Event details expose only an authorized opaque trace ID. A same-origin viewer route rechecks record access and uses a fixed configured viewer. Historical or absent trace states remain explicit. Raw errors, framework spans, baggage, trace state, arbitrary resource attributes and financial values are excluded. API 3.6.0 documents the optional response field; generated clients and examples are aligned.

Verification: All 43 Go packages and Go vet passed. The full isolated integration suite passed 136 top-level tests and 83 subtests; the standalone crash helper intentionally skipped and its parent passed. Six fault tests passed. Final affected platform and contract checks passed after span-lifetime and API-contract changes. The frontend passed 220 tests, TypeScript and targeted ESLint. Seven developer contract checks passed after version alignment. Nineteen operations/diagnostic browser checks passed; the final BFF runtime production build and three diagnostic journeys also passed. Three 10,000-operation local telemetry microbenchmarks recorded bounded collector-down overhead. Collector YAML is parsed locally; live collector compatibility remains in R17 package verification.

Preservation: Every phase-entry source file remains. The Git index and HEAD are unchanged. Migration 000061 ran only in the verified disposable PostgreSQL stack. Normal financial fingerprints match R04 and all five preserved services are healthy. No normal migration, financial command, image rebuild or cloud execution ran.

Issues and limits: The BFF span measures private-context preparation, not the complete browser request. Traces are sampled diagnostic evidence; sampling, retention and collector loss can remove them. Collector configuration was parsed; live collector compatibility and a configured private viewer remain package/deployment checks. Existing locked telemetry dependency versions were promoted without a version upgrade; R13 owns final dependency review. Normal images remain preserved prior images. No normal cutover or managed-production qualification is claimed. R07-R17 remain selected implementation work. See [completion evidence](../release-evidence/portfolio/r06/completion.json). R07 is next.


## R07 completion — 15 October 2026

A host-only runner now executes seven fixed failure scenarios in a new tmpfs PostgreSQL/Redis project. It verifies exact IDs, labels, images, storage and loopback binds before each scenario and cleanup. It compiles and preserves the actual Go test binaries, source/index hashes, applied migration identity, limits and JSON test output. The verifier rejects missing, skipped or failed tests, tampered artifacts and mismatched provenance. Cancellation retains an incomplete bundle and verifies cleanup. The advanced Systems Lab reads only an archived safe summary; it has no browser fault command.

Verification: The final run demo-20261015111612-82a0f63d passed lost response, concurrent duplicate command, duplicate/reordered event, stale worker, cache loss, publication dependency loss and real worker process crash scenarios. Its 26 artifacts total 75,953,051 bytes and pass integrity/provenance checks. Both temporary containers were removed. An independent startup cancellation run exited nonzero, retained incomplete/failed checks and passed cleanup. All 31 contract/safety/projection tests, 220 frontend tests, TypeScript, targeted ESLint, a production build and 11 developer/Systems Lab browser journeys passed. The quality workflow parses and includes the evidence safety tests. No hosted CI run is claimed.

Preservation: All phase-entry files remain, with unchanged Git index and HEAD. Each complete runtime run checked source/index and normal container identities. Normal financial fingerprints match before and after the final run, and five preserved services remain healthy. No normal migration, seed, financial command, image rebuild or cloud action ran. Failed, cancelled and successful bundles remain separate.

Issues and limits: Source/binary provenance belongs to each recorded run; later documentation and generated summary updates do not relabel old runs. The lost-response case discards a service result; no network-proxy or multi-host partition claim is made. PostgreSQL/Redis image IDs are dependency identities. No candidate API image was built or tested in R07. The browser receives only the safe historical projection. Optional-surface availability remains R11. R08-R17 remain selected implementation work. See [completion evidence](../release-evidence/portfolio/r07/completion.json). R08 is next.


## R08 completion — 15 October 2026

An independent standard-library financial model now predicts exact balances, versions, durable outcomes and journals before commands run. Five seeded sequences cover transfers in both directions, rejected/replayed/conflicting intents, dual-reviewed funding, approved exact corrections, lost caller results, pre-commit rollback and four-way duplicate concurrency. Every step checks database balances, versions, balanced journals and immutable old records through a restricted API workload login. The guarded R07 host harness now has a fixed model profile and bounded counterexample replay. The minimizer and reproducible seed/operation records are retained.

Verification: The full isolated integration suite passed 137 top-level tests and 88 subtests; the standalone crash helper intentionally skipped and its parent passed. All 44 Go packages and Go vet passed. The final fresh guarded model profile passed five seeds and 320 steps, with 214 balanced synthetic journals across those sequences. A new isolated run replayed the historical one-step harness input successfully. Both runtime bundles have 13 verified artifacts, exact compiled-binary/source/migration provenance and verified cleanup. Standard-library dependency and minimizer checks passed. The final ten-second fuzz run passed with two workers. All 34 host input/evidence safety tests passed. CI syntax and model vet/unit/race/fuzz/replay steps were checked locally; no hosted run is claimed.

Preservation: All 1,694 phase-entry source files remain. Git index and HEAD are unchanged. Financial commands and test-only failure triggers ran only in verified disposable databases. Normal financial fingerprints still match R07 and five preserved services are healthy. No production financial code, normal migration, financial command, image rebuild or cloud execution changed.

Issues and limits: The model covers bounded valid USD intents, selected workflows and selected concurrency. It does not prove every schedule or policy. Generated correction steps with no eligible funded original are checked no-ops. The full integration run and final guarded model run are retained separately; each exact-source runtime bundle states its own candidate identity. No product money-movement defect was found in these bounded runs. The initial receipt mismatch was a corrected harness assumption, with failure evidence retained. Normal runtime images remain preserved prior images. No normal cutover or managed-production claim is made. R09-R17 remain selected implementation work. See [completion evidence](../release-evidence/portfolio/r08/completion.json). R09 is next.


## R09 completion — 15 October 2026

A guarded local capacity profile now drives fixed offered load through the real financial application service and a real native continuous worker. It measures hot and independent account pairs, two tenants, exact replay/rejection outcomes, authoritative reads, latency distributions, queue growth, pool/lock samples and bounded resources. Receipt storage reuses one scoped transaction for consecutive messages from the same tenant; every message retains its controlled function and Lamport checks. Publication preparation reads diagnostic origin in its existing query. Three same-source baseline calibrations and two three-run candidate cohorts retain favourable and limited results. Default batch 10 remains. Three final default runs qualify 10 offered commands per second for the stated component workload, followed by actual Redis interruption and worker process restart. A separate real signed slow TLS receiver check proves posting and lease renewal continue while HTTP waits.

Verification: All 44 deterministic Go packages and Go vet passed. The complete isolated integration run passed 138 top-level tests and 88 subtests; the standalone crash helper and opt-in capacity test skipped as designed. A separate added slow-receiver integration test passed 12 postings with p95 170.64 ms, a renewed lease and a free one-connection worker pool. Six fault tests passed. Each of three full capacity runs completed a requested 300-second, 3000-command steady stage, then a 60-second, 300-command recovery stage. Each complete run produced 3168 journals and passed exact oracle balances/versions, balanced postings, original/replay identity, internal reconciliation, projection convergence, source/index/normal-container preservation and cleanup. Steady command p95 was 27.27–27.63 ms; sampled steady oldest pending was 0.41–2.10 seconds. All 28 artifacts per run passed a separate integrity recheck. A PID-bound cancellation check proved an actual native worker stopped, no capacity pass was claimed, and both temporary containers were removed. All 37 final host safety tests passed. CI syntax and wiring passed locally; no hosted run is claimed.

Preservation: All 1706 phase-entry source files remain. Git index and HEAD are unchanged. Normal financial fingerprints match R08 and all five preserved services are healthy. All test mutations, claims and faults used verified disposable targets. Three older performance/decision documents retain their full prior text and now identify their historical measurement scope. Failed, limited, successful and cancelled records remain separate. No normal migration, seed, financial command, image rebuild, volume replacement or cloud execution ran.

Issues and limits: One local synthetic Go-service/native-worker component study. It does not measure HTTP/BFF throughput, candidate API images, multiple hosts, managed infrastructure or a production SLA. Offered commands include replays and expected rejections. Rejected response counts overlap replay counts; the capacity note provides exclusive new-outcome counts. 20 offered commands per second remains unqualified. Short calibrations never qualify a five-minute interval. Batch 50 has no sustained capacity recommendation; default 10 remains. Driver CPU/current heap and PostgreSQL/Redis resources are recorded. Native worker CPU/RSS are not measured here; R13 must inspect final service resources. Queue, connection and lock statistics are sampled. The slow-receiver profile is a finite 12-command burst and one controlled TLS receiver with a test-only transport redirect. Production destination policy is unchanged. The cancelled and early failed bundles are incomplete evidence, not successful financial qualifications. The early paused target was removed only after an independent exact-identity and original-port check. The three measured bundles bind their exact dirty source and compiled binaries. Later PID-bound cancellation metadata, summary-profile checks, documentation and phase records do not relabel that measurement build. Production Go code is unchanged after the qualified measurements. Normal runtime remains on preserved prior images. No normal cutover or cloud/commercial production approval is claimed. R10–R17 remain selected implementation work. See [completion evidence](../release-evidence/portfolio/r09/completion.json). R10 is next.


## R10 completion — 15 October 2026

A fixed local recovery profile now backs up an unfiltered synthetic candidate, validates bounded source/schema/checksum metadata, restores into a distinct temporary PostgreSQL target, reapplies restricted grants, and checks complete row fingerprints before new commands. It verifies original receipts, unresolved recovery, approved replay, exact financial invariants, reconciliation, cache rebuild and real worker progress. Separate migration checks cover the oldest supported local snapshot, known legacy room history, the previous ordered candidate, real transaction cancellation/retry, and bounded retention. Recovery displays a separate authorized read-only candidate proof.

Verification: The final run demo-20261015140934-45e3fe02 passed all three named scenarios, source/index/normal-container preservation, and exact cleanup. All 57 public tables matched counts and canonical row hashes before restored writes. Three exact disposable containers were removed. All 29 artifacts, totalling 44,838,990 bytes, passed a separate verification. Backup took 1.405 seconds; restore setup took 5.487 seconds; backup through all named financial and migration checks took 20.75 seconds. All 44 deterministic Go packages, Go vet, 39 host safety/backup/projection checks, 220 frontend tests, TypeScript, targeted ESLint, a production web build and eight Recovery/export browser checks passed.

Preservation: The authorized Git checkpoint is 65e45ae. The original pre-commit R10 baseline remains, and a separate resumed baseline binds the new HEAD/index. All 1729 resumed source files remain. Normal financial fingerprints match R09 and all five preserved services are healthy. No normal migration, seed, financial command, image rebuild, volume replacement or cloud action ran. Failed and successful bundles remain separate.

Issues and limits: This is a one-host synthetic Go-binary recovery proof, not a candidate API image or managed point-in-time recovery claim. The normal-runtime backup index remains separate; a passing candidate drill does not establish current normal backup coverage. The supported old snapshot is the existing phase-seven migration fixture; arbitrary historical databases are not qualified. Two failed harness runs remain: the first rejected a valid explicit pg_temp-last search path; the second passed financial checks but closed the admin pool before ephemeral-role cleanup. Both were repaired and final checks passed. Documentation and public summary updates occur after the recorded run; they do not change its source identity. Repeat the complete drill after the selected statement and reservation schema changes. R11-R17 remain selected work. See [completion evidence](../release-evidence/portfolio/r10/completion.json). R11 is next.


## R11 completion — 15 October 2026

Optional features now use strict server-owned core, study and collaboration sets. The API and BFF deny disabled active routes, and the session drives Systems Lab visibility. Authorized old room links use a separate scoped read-only history reader with no expiry, audit, presence or signalling writes. The cron adapter owns a separate database pool and verifies an exact restricted worker LOGIN before it can claim work. Base Compose and examples default to core; dedicated overlays retain study and collaboration. Deferred payouts and denied Administration remain preserved.

Verification: The final disposable profile demo-20261015142740-0ff82ac1 passed restricted cron execution, retained room history with no data/audit changes, enabled collaboration lifecycle, ephemeral Redis signals, scheduler fencing and scheduler tenant isolation. All 15 artifacts passed a separate integrity check and both exact temporary containers were removed. All 44 deterministic Go packages, Go vet, 221 frontend tests, TypeScript and targeted ESLint passed. A production build and three browser checks passed core visibility, direct BFF denial, retained findings without active requests, and the selected study display. All three Compose feature sets rendered with matching API/BFF flags; no rendered configuration was applied.

Preservation: All 1739 phase-entry files remain, with unchanged HEAD/index at the authorized 65e45ae checkpoint. Normal financial fingerprints match R10, and all five preserved services remain healthy. No normal migration, financial command, image rebuild, volume replacement, feature toggle or cloud execution ran. Historical room data, migrations, specifications and previous evidence remain.

Issues and limits: The normal runtime retains its earlier images and active settings; changed Compose defaults were rendered only. Study and collaboration are explicitly separate development-only sets; combined flags and invalid values fail configuration. Existing participant, tenant and scope checks remain on room history; disabling the active feature does not erase or grant access to evidence. Local database/Redis, handler and browser checks do not claim a physical cross-network WebRTC call, managed TURN or arbitrary optional combinations. R13 owns final integrated image qualification. The first study browser fixture omitted selected feature flags and the first overlay edit duplicated its web mapping. Both failures were retained, corrected and reverified. No source or data was removed to simplify the default experience. R12-R17 remain selected work. See [completion evidence](../release-evidence/portfolio/r11/completion.json). R12 is next.


## R12 completion — 15 October 2026

One dependency-free TypeScript backend example now persists complete exact intents, operation/destination and original credential bindings before send, supports explicit original-key recovery across processes, and receives signed events with durable deduplication. The generated client preserves the API base prefix, rejects escaped/plaintext remote destinations, bounds network time, protects credential headers, and can obtain a fresh delegated assertion per call. API 3.7.0 documents own-principal versus delegated authentication. Developer starts with this workflow and derives operation-specific retry advice from a generated web-local contract projection. Other recipes and detailed references remain available.

Verification: The clean disposable run demo-20261015150744-f551eea6 passed real HTTP request authentication, restricted API/worker roles, exact-zero account creation, independent synthetic finance review/posting, actual successful-response loss and original retry from a new Node process, 409/400/403/429 responses, authoritative read-after-write and reconciliation. It proved two balanced journals/four postings, actual signed endpoint verification, real worker delivery failure with HTTP 503 and later recovery, and duplicate acknowledgement after receiver process termination/restart with one durable business effect. The named flow took 5.254 seconds; the full host bundle interval was 32.361 seconds. All 32 artifacts passed integrity checks and both exact containers were removed. All 44 deterministic Go packages, Go vet, four example tests, example TypeScript, 39 host checks, 222 frontend tests, frontend TypeScript, targeted ESLint, a production web build and 11 Developer browser journeys passed. Offline install, generated drift and workflow syntax checks passed. CI wiring is added; no hosted result is claimed.

Preservation: All 1753 phase-entry files remain, with unchanged HEAD/index at 65e45ae. Normal financial fingerprints match R11 and five preserved services remain healthy. Synthetic SQL setup, commands and receiver faults used verified disposable targets; normal configuration/images/data were not changed. Failed profiles and browser/guard failures remain separate from final passing evidence. No cloud action or new push ran.

Issues and limits: This clean environment uses new disposable databases/configuration and separate Node processes with the installed pinned runtime. It does not claim a second human review, fresh-machine installation, external OAuth provisioning or a candidate API image. The local fixture uses real RequestAuthenticator/handlers with a synthetic own-principal identity provider and restricted workload pools; final provider/image qualification remains separate. Receiver rewriting is confined to a fixed synthetic HTTPS destination and an owned loopback process in Go test code. Production SSRF/transport policy is unchanged. Real delivery selects transfer.posted. Balance-event ordering is a separately labelled signed synthetic fixture; no new subscription availability is claimed. Node SQLite is experimental in pinned Node 24.12.0 and is limited to this local example. It is not the ledger authority or an approved shared production receiver store. The latest integration source snapshot remains immutable. Subsequent OpenAPI wording and generated EOF normalization are documentation/format corrections, with final drift, type, Go and frontend checks; they do not relabel that snapshot. R13-R17 remain selected work. See [completion evidence](../release-evidence/portfolio/r12/completion.json). R13 is next.


## R13 resumed checkpoint — 15 October 2026

The user-authorized main update is complete at 67d7a6c. Original local commits, changes, source backups, database backup and private evidence remain. All 1777 original R13 paths remain. A separate 1881-file source/index checkpoint and the updated migration-000062 normal fingerprint now bind resumed work. The original eight accounts, five transfers, 20 postings and all money hashes match R04. The other chat passed six fresh-stack journeys, 242 web checks and the normal browser flow. It repaired feature-aware study checks, per-test screenshots, opaque-session propagation, fresh session grants and stylesheet layer order. R13 is active; no final post-update qualification is claimed yet. The candidate runner now retains browser outputs in its own unique private directory. R14–R17 remain selected and unstarted.
