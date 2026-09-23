# Database workload capability matrix

Status: executable baseline for PR-005

Matrix version: `2026-09-03.pr005.v1`

Control owner: platform security and database engineering

## Purpose

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

## Current hardening gaps

The harness intentionally captures these known exposures without normalizing them as the desired design:

- API can directly insert or update accounts and projections and directly insert openings, ownership, journals, postings and audit events.
- Worker and reconciliation can directly insert audit events; reconciliation can directly read unscoped financial history.
- Provisioning can directly create or update account state, create opening/projection/ownership rows, delete ownership and insert audit events.
- API, reconciliation and support can read another tenant's account rows because transaction tenant context and RLS are not yet enforced.
- Workloads with `USAGE` on `public` can reach internal trigger-function execution ACLs inherited from `PUBLIC`.

Break-glass remains a negative control for table DML, cross-tenant reads, public-schema function invocation and schema creation. Schema creation is also denied for every other workload login; the migration owner proves the positive control.

## Probe coverage

For each of API, worker, reconciliation, provisioning, support and break-glass, the harness checks:

- `SELECT`, `INSERT`, `UPDATE` and `DELETE` on `accounts`, `account_opening_balances`, `account_balance_projections`, `journal_transactions`, `ledger_postings`, `account_owners` and `audit_events`;
- a cross-tenant account query with transaction tenant context set to another tenant;
- invocation authority for `public.reject_ledger_mutation()`;
- `CREATE SCHEMA` in the target database;
- login safety flags, session identity, RLS enabled, exactly one intended inherited workload role, no sibling memberships and no direct table grants.

All mutation and schema probes run inside transactions that are rolled back. An allowed `INSERT DEFAULT VALUES` commonly reaches a constraint error; reaching that constraint proves PostgreSQL accepted the table privilege. Permission denial (`SQLSTATE 42501`) is the only denied result. Other unexpected SQL states fail the harness so schema drift cannot masquerade as least privilege.

The same matrix runs against both a fresh migrated database and the supported phase-seven-to-current upgrade path. The migration compatibility test therefore catches upgrade-only grant drift.

## Planned convergence

| Plan step | Capability change |
|---|---|
| PR-008 | Add narrowly scoped, tenant-aware financial and opening-import functions before revoking existing writes |
| PR-009 | Revoke general financial DML, internal function execution and database schema creation from workload sessions; flip the corresponding current expectations to denied |
| PR-010 | Set transaction-local tenant context and enable pilot RLS; flip cross-tenant read expectations to denied |
| Production qualification | Validate the same allow/deny matrix using externally issued workload credentials and retain the redacted artifact with release evidence |

Any matrix change requires a reviewed edit to the version, current expectation and target rationale in the same pull request as the grant or migration change. Never weaken the target to make a newly observed grant pass.
