# Module ownership and disposition

Baseline: [R00 source identity](baseline.md). Exact file lists, imports, route declarations, table declarations, test names, optional references, and bounds are in [source-inventory.json](source-inventory.json). This is an inventory of existing source. It is not a list of features to rebuild.

## Application packages

Every existing application package is listed below. The package name is relative to `internal/application/`. Data ownership means ownership of the use case; SQL mechanics remain in `internal/platform/db`.

| Package | Plain-language job | Records or boundary | Decision and next phase |
|---|---|---|---|
| accounts | Create and manage accounts; read balances | Accounts, owners, balance projections | Keep; R02/R03 qualify authority. |
| approvals | List work that needs a separate decision | Approval records, funding/correction requests | Keep existing inbox; R05 simplify access. |
| consistency | Carry minimum balance-version requirements | Read-your-writes contract | Keep; R04 preserve exact version behaviour. |
| corrections | Request and approve compensating transfers | Correction records and new linked transfers | Keep; R03 complete controlled writes. |
| delivery | Read and retry delivery evidence | Delivery attempts and replay actions | Keep; R04 clarify owner versus webhook jobs. |
| developerplatform | Manage developer credentials and endpoints | Credential, webhook, and command-idempotency records | Keep; R12 use through an independent client. |
| exports | Stream bounded evidence files | Authorised read interfaces | Keep; R02 cover export isolation. |
| funding | Record authorised funding and approval | Funding events, policies, approval records | Keep; R03 controlled command boundary. |
| guidance | Store onboarding preferences | Operator preferences | Keep small; R05 consolidate explanations. |
| idempotency | Define stable command identity and replay | Request identity/outcome contract | Keep; R03/R08 test conflicts and retry. |
| investigation | Search, save, and share investigation evidence | Saved views, workspaces, optional live rooms | Keep search/saved evidence; live rooms remain optional. |
| lease | Renew and fence a worker claim | Worker/store callback boundary | Keep; R07/R08 prove stale-worker rejection. |
| operations | Present event and webhook operations | Outbox, delivery, operational reads | Keep; R04 review presentation coupling. |
| outbox | Publish committed events with retry | Outbox events and publication attempts | Keep; R03/R07 preserve at-least-once contract. |
| payouts | Describe simulated provider behaviour | Payout interfaces and fake provider | Isolate; no real payout claim. R11. |
| pilotgate | Express historical release prerequisites | Pilot readiness evidence | Retain history; reduce default prominence in R05. |
| projection | Apply monotonic balance observations | Redis/cache boundary | Keep; PostgreSQL remains authority. R07/R08. |
| provisioning | Provision authorised partner resources | Tenant roles, provisioning requests, credential events | Keep bounded; R03/R12 qualify. |
| reconciliation | Compare postings and internal projections | Reconciliation runs/mismatches/commands | Keep and label internal; R15 external source is conditional. |
| recovery | Present retained recovery evidence | Backup and restore metadata | Keep; R10 prove isolated restore. |
| retention | Run explicit evidence-retention work | Retention runs and policy limits | Keep; R10/R13 protect required evidence. |
| scheduler | Claim scheduled reconciliation work | Scheduler tenants, leases, jobs | Keep explicit enablement; R07/R11 qualify fencing. |
| transactions | Read account transaction history | Tenant/account authorised history | Keep; R02 verify paging and joins. |
| transfers | Validate and submit exact transfer intent | Controlled transfer repository contract | Keep; R03 preserve existing controlled transfer path. |
| webhookdelivery | Deliver to approved endpoints | Webhook delivery jobs | Keep; R04 extract HTTP mechanics only if useful. |
| webhookverification | Verify endpoint control before delivery | Verification jobs and endpoint state | Keep; R03/R12 qualify authority and failure. |

## Other Go package boundaries

Domain packages are `account`, `funding`, `identifier`, `ledger`, `money`, `payout`, and `transfer`. They own values and invariants. `identifier` currently has a SQL value-conversion exception. `internal/deviceevidence` owns the physical-device evidence utility; it does not certify a device review by itself.

Platform packages are `cache`, `config`, `db`, `events`, `identity`, `observability`, `recovery`, `redisconn`, `routing`, `secrets`, and `startup`. They own infrastructure adapters. HTTP transport, handlers, and middleware own request boundaries. Bootstrap and commands compose processes. The inventory maps every production package to its files and imports.

Current Go package discovery found no import cycle. Do not infer that logical coupling is absent. The largest authored files include transfer views (925 lines), correction console (912), console shell (593), funding repository (554), live investigation repository (468), operations repository (465), telemetry (465), and correction repository (459). Size identifies review candidates, not automatic defects. R04 must split by responsibility and preserve public contracts.

Potential duplicate responsibilities include delivery replay versus webhook job operations, repeated repository transaction/authorisation setup, and state handling in large console controllers. These are design concerns. Compare behaviour before extracting a shared component. The existing lease package is already a shared worker boundary; do not create a second one.

## Console and routes

The existing feature folders are accounts, approvals, auth, console, corrections, developer, exports, funding, guide, investigation, operations, orientation, overview, reconciliation, recovery, and transfers. Keep feature-specific controllers and views. Shared UI, API clients, and styles remain in their existing layers. The route inventory contains BFF handlers and UI pages. ADR-004 defines the compatibility map.

Approvals and Webhooks have source and passed representative existing-image page smoke. Developer has contract tests. Styles already have separate foundations and feature files. Administration remains absent. None of these observations permits closure of an unrelated production or human-review gate.

## Optional code and callers

Live investigation references occur in Go configuration, API registrations, handlers, repositories, room migrations, browser boundaries/features, contracts, tests, and docs. Preserve stored room records and links. The server feature defaults off and existing tests exercise disabled denial. A frontend visibility check alone is insufficient.

Fake payout references belong to provider simulation, application/domain contracts, composition, and tests. They must not become an external payment claim. Study routing, peer observations, Lamport metadata, and snapshot/fault commands have separate source, tests, scripts, and documentation. Keep these for the Systems Lab. The scheduler has explicit server enablement and durable lease/job tables.

Use `optional_references` in the JSON inventory to locate exact callers, including generated-contract-adjacent sources and documentation. Generated client files were excluded from the lexical scan to avoid duplicate output; `contracts/generated/` remains in the full source manifest and must be regenerated/checked if an optional public contract changes. Dynamic dispatch and runtime configuration need explicit review in R11.

## Background processes and data

The normal runtime has API, outbox worker, web, PostgreSQL, Redis, and completed migrate/demo-seed jobs. The worker contains outbox, webhook delivery, and verification responsibilities. Scheduler and study processes require their explicit configuration. R07 tests failures; R11 reviews supported capability combinations.

SQL declarations include core accounts/journal/postings/idempotency/outbox; audit, policies, velocity and rate limits; approvals/funding/corrections; credentials/webhooks/verification; investigation/workspaces/live rooms; recovery intent; logical clocks; and scheduler leases/jobs. The full declaration list includes locations and repeated historical migrations. It is not an assertion that 65 distinct tables are live.

## Test and documentation map

Keep domain and application unit tests near their packages. `tests/unit` covers core behaviour, `tests/contract` covers public contracts, and integration/fault/system directories require their documented dependencies. Web UI/security tests are fast source checks. Playwright tests cover browser and real-stack boundaries. `tests/architecture` now guards imports. The demo schema tests run offline.

Use existing architecture, security, local-runtime, release-evidence, and plan documents as inputs. R00 adds a reconciled baseline; it does not replace historical evidence. The issue register assigns unfinished validation to later phases.
