# Database workload capability matrix

Consolidation update: migration 000064 adds immutable, approved opening-import evidence. All four new tables force tenant row policies. Only provisioning can execute the three import functions; API, worker, support and other workload execution remains denied. Each function checks the current tenant and actor. The complete ACL inventory and fresh/populated database suites passed on disposable targets. The older R03 counts below describe that phase's recorded checkpoint.

R13 candidate update: migration 000063 adds forced row security for opaque BFF sessions and UI preferences. Opaque lookup occurs before tenant discovery, so its exact token digest is installed only for the owning transaction. Preference rows require both tenant and actor context. A separate fixed cleanup function removes at most 100 expired or long-revoked records for the current operator and returns only a count. Raw API deletion is revoked. The controlled function execution inventory includes this non-financial cleanup function. These changes are qualified on disposable targets. They are not applied to the preserved normal database in this work packet.

Status: R03 locally verified through migration 000060 on 15 October 2026. All current core observations align with the target.

Matrix version: `2026-10-15.r03.v1`

Control owner: platform security and database engineering

## Purpose

The current role script removes direct financial and audit writes from the six workloads covered by this matrix. It also removes PUBLIC execution of internal trigger helpers. Real workload logins pass the current and target core table checks on fresh and populated upgrades. R02 enforces scoped row policies under the documented trusted-workload limit. Controlled recovery, reconciliation evidence and maintenance paths now pass their restricted-role and negative tests. See the complete R03 evidence; this core matrix remains only one part of acceptance.

This matrix records two separate truths:

1. **Current allowed** is the authority that `deploy/postgres/roles.sql` grants today, including unsafe authority that later pull requests must remove.
2. **Target allowed** is the production boundary: workloads may read only their approved scope and must perform financial changes through enumerated, tenant-aware functions.

`tests/integration/database_role_capabilities_test.go` is the machine-enforced source of truth. It creates a cryptographically random login for each workload role, opens a real PostgreSQL session, verifies safe login attributes and exclusive role membership, and executes rollback-safe probes. CI fails if observed authority differs from the versioned current baseline. A known current capability whose target is denied is emitted as `hardening_gap`; an unreviewed grant or revocation is emitted as `unexpected_drift` and fails the test.

The JSON CI artifact contains role names and boolean capability results only. It never contains connection strings, passwords, tenant identifiers, or row data.

## Target role boundary

| Workload role | Reads retained at target | Direct writes retained at target | Replacement boundary |
|---|---|---|---|
| API | Accounts, opening balances, projections, journals, postings, ownership and audit, all tenant-scoped | None on financial or audit tables | Enumerated account, transfer, funding, correction and audit functions |
| Worker | None of the core tables in this matrix | None | Enumerated outbox/webhook claim and finalize functions; a dedicated projection function only if required |
| Reconciliation | Tenant-scoped accounts, opening balances, projections, journals and postings | None | Reconciliation run, mismatch and audit functions |
| Provisioning | None of the core tables in this matrix | None | Zero-opening provisioning function and separately approved non-zero import function |
| Support | Masked, ticket-bound account, projection, journal, posting, ownership and audit reads | None | Read-only views with separately approved elevation |
| Break-glass | None while dormant | None | Time-bounded external assumption with explicit incident grants |

Every target session also denies cross-tenant account reads, arbitrary execution of internal trigger functions, and schema creation. The migration owner is the positive control for schema and object authority; it is not a workload login.

## Final local qualification

The final report contains 294 aligned observations. It checks seven workload roles on fresh installation and populated upgrade, including protected-table TRUNCATE denial. A separate allowlist verifies EXECUTE rights for every controlled function, including denied administrative import and fingerprint helper access. Catalog checks verify non-login owners, fixed search paths, no PUBLIC execution and safe function defaults. Actual command tests verify permitted use and hostile input. The restricted maintenance role receives only its bounded retention function.

See [R03 completion](../release-evidence/portfolio/r03/completion.json), [current metadata](database-access-r03-final.json), and [cutover procedure](controlled-write-cutover.md). The following checkpoint and gap history is retained for traceability.

## Historical checkpoint and gaps

At checkpoint 000054, API account, opening, projection, journal, posting and ownership writes are denied. Provisioning has no direct core table writes. Worker and reconciliation audit INSERT is denied. Controlled account, funding, correction, provisioning, audit and fenced worker paths passed the complete integration suite. Current trigger-helper EXECUTE is false for all workload roles. Break-glass remains dormant. [Checkpoint evidence](../release-evidence/portfolio/r03/checkpoint-000054.json) records the verified source and logs.

The following paragraphs retain the earlier gap history. The bullets describe the R02 state, before R03 revocation. They are not the current grants.

Migration `000040` adds `transfer_recovery_intents`. The API receives `SELECT`
and `EXECUTE` on `controlled_prepare_transfer_recovery_v1`; it receives no direct
`INSERT`, `UPDATE`, or `DELETE` on the intent table. The non-login migration owner
can insert through the controlled prepare function. An immutable trigger rejects
updates and deletes. Recovery list/get queries bind the authenticated tenant and
original actor and check current source and destination authority. This addition
does not close the broader direct financial DML or RLS gaps listed below.

The harness intentionally captures these known exposures without normalizing them as the desired design:

- API can directly insert or update accounts and projections and directly insert openings, ownership, journals, postings and audit events.
- Worker and reconciliation can directly insert audit events; reconciliation reads now require tenant/actor context.
- Provisioning can directly create or update account state, create opening/projection/ownership rows, delete ownership and insert audit events.
- Cross-tenant account reads are now denied for API, reconciliation and support. The current expectation is false. Missing scope also returns no tenant rows.
- Workloads with `USAGE` on `public` can reach internal trigger-function execution ACLs inherited from `PUBLIC`.

Break-glass remains a negative control for table DML, cross-tenant reads, public-schema function invocation and schema creation. Schema creation is also denied for every other workload login; the migration owner proves the positive control.

## Probe coverage

For each of API, worker, reconciliation, provisioning, support and break-glass, the harness checks:

- `SELECT`, `INSERT`, `UPDATE` and `DELETE` on `accounts`, `account_opening_balances`, `account_balance_projections`, `journal_transactions`, `ledger_postings`, `account_owners` and `audit_events`;
- a cross-tenant account query with transaction tenant context set to another tenant;
- invocation authority for `public.reject_ledger_mutation()`;
- `CREATE SCHEMA` in the target database;
- login safety flags, session identity, RLS enabled, exactly one intended inherited workload role, no sibling memberships and no direct table grants.

All mutation and schema probes run inside transactions that are rolled back. INSERT probes now use `EXPLAIN (FORMAT JSON) INSERT ... DEFAULT VALUES` to measure the grant without confusing an invalid row with row-policy rejection. Actual valid scoped writes, rejected foreign writes and worker behavior have separate integration tests. Permission denial (`SQLSTATE 42501`) is the only denied result. Other unexpected SQL states fail the harness so schema drift cannot masquerade as least privilege.

The same matrix runs against both a fresh migrated database and the supported phase-seven-to-current upgrade path. The migration compatibility test therefore catches upgrade-only grant drift.

## Planned convergence

| Plan step | Capability change |
|---|---|
| PR-008 | Add narrowly scoped, tenant-aware financial and opening-import functions before revoking existing writes |
| PR-009 | Revoke general financial DML, internal function execution and database schema creation from workload sessions; flip the corresponding current expectations to denied |
| PR-010 | Set transaction-local tenant context and enable pilot RLS; flip cross-tenant read expectations to denied |
| Production qualification | Validate the same allow/deny matrix using externally issued workload credentials and retain the redacted artifact with release evidence |

Any matrix change requires a reviewed edit to the version, current expectation and target rationale in the same pull request as the grant or migration change. Never weaken the target to make a newly observed grant pass.
