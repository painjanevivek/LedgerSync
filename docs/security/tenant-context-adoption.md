# Tenant database context: adoption record

Updated: 15 October 2026. R02 is locally verified. R03 has started with writer inventory. The security claim is limited to trusted workload scope and actual tested policies; it excludes full workload compromise.

## Completion

Migrations 000044–000046 add bounded queue discovery, forced row policies on all 53 tenant-bearing or related tables, and controlled verification completion. All identified repository tenant paths now install context. Export, investigation, diagnostics and credential-use interfaces carry the authenticated actor. Maintenance uses named system actors. Worker item processing resolves a live fenced claim, installs its tenant, and checks the fence again before each state change. A committed multi-stage command releases its reserved connection before its next stage.

The verification worker keeps no direct endpoint UPDATE grant. Its controlled function activates one pending endpoint and appends one event atomically. A delivery timestamp typing defect was also reproduced and repaired. Public function execution, owner membership and unsafe role attributes are checked. The new role setup rejects unsafe bindings.

Final checks: 119 integration tests, 81 subtests, six fault tests, 41 Go test packages, and Go vet passed. Fresh and populated upgrade paths passed. One standalone crash-helper skip is intentional; its parent passed. Tests now preserve permanent policies and include forced non-superuser owner behavior, malformed context, direct foreign projection IDs, joins, unsafe inheritance, and all worker paths. Normal financial fingerprints match the start of this continuation. Race/image/browser/restore qualification remains outside this phase.

See [cutover and authority exceptions](tenant-context-cutover.md), [final access inventory](database-access-r02-final.json), and [completion evidence](../release-evidence/portfolio/r02/completion.json). The sections below preserve the earlier checkpoint for audit context. Their remaining-work statements are historical.

## Historical checkpoint

## What changed

The database adapter now has an explicit `TenantScope`. It contains the tenant and actor from an authenticated server boundary. The helper rejects missing or malformed values before it reserves a connection. A browser header does not establish this scope.

`WithTenantTransaction` owns one transaction. `WithTenantSerializableSequence` retains the existing advisory lock name, serializable isolation, and bounded retry behavior. It installs the scope again for each attempt. Read adapters use a read-only, repeatable-read transaction. Nested evidence queries use that same transaction. They do not reserve another connection while holding the first connection.

Before a scoped operation starts, the helper clears any old session scope. After the operation ends, it clears the session again. Cleanup uses a fresh context with a two-second limit. If cleanup fails, the connection is discarded. Cleanup does not report a known committed financial result as rejected.

This second cleanup step is necessary. PostgreSQL restores an earlier session value after a transaction-local setting ends. A test first reproduced this leak. The repaired helper passes the same test, including a terminated database connection.

## Current coverage

| Access path | Adopted behavior | Remaining work |
|---|---|---|
| Account list/detail, balance authorization/current read, account history | Tenant and actor on one read transaction | Tenant-wide maintenance balance reads |
| Account lifecycle commands and denial audit | Scope inside command transaction and separate required denial audit | Controlled writer cutover and privilege revocation in R03 |
| Transfer submit, recovery preparation/list/detail | Scope preserved through retry and durable recovery preparation | Row policies and final capability qualification |
| Funding and corrections | Scoped command transactions, authorized reads and funding reconciliation | Remaining writer authority in R03 |
| Approvals, event evidence and webhook evidence | Scoped reads; causal evidence shares the event transaction | Operations summary still needs an actor-aware interface |
| Investigation search/relationships, saved views, workspaces and live rooms | Scoped reads and writes; workspace reference authorization shares the current connection | Legacy transfer/reconciliation reads and exports |
| Guidance and onboarding preferences | One snapshot for each read; versioned preferences use a scoped write transaction | Final policy activation and upgrade proof |
| Developer credential and webhook commands and reads | Explicit actor passed from HTTP principal through service and repository | Credential usage recording and global delivery/verification workers |
| Workers, provisioning, scheduler, retention, reconciliation and global infrastructure | Existing behavior preserved | Narrow global entry points, item scope, maintenance identities and full role matrix |

## Database inventory

[database-access-inventory.json](database-access-inventory.json) records the schema and role metadata from the isolated migrated database before row policies are added. It contains 57 public tables, 49 tables with a tenant column, 68 function records, eight named workload/owner roles, and 241 table grants. No table had row security enabled. Table owners, function owners, function search paths, public execution, memberships, views and default privileges are included.

The inventory is a local baseline. It is not a measurement of a hosted database. Existing API financial table grants remain broad. Passing the existing capability test does not close that gap.

## Verification

The tests use a new PostgreSQL container and a new Redis container. Both use temporary storage and separate loopback ports. The runner checks project labels, image identities, and database mounts before migration or tests. Normal local volumes are not mounted.

The new tests exercise actual restricted API logins. They cover missing scope, invalid scope, tenant A/B isolation, actor propagation, cross-tenant writes, commit, rollback, cancellation, panic, concurrent pool reuse, serializable retry, stale session settings, and failed connection cleanup. Guidance and developer tests enable test-only row policies and check reads on a pool with one connection. This detects hidden queries that escape the transaction or wait for a second connection.

The complete integration run passed 113 top-level tests and 73 subtests. The standalone crash-helper test skips by design; the process-crash parent test invokes it and passed. All six fault tests passed. Internal Go, unit, contract and architecture packages passed after two local HTTP test packages were rerun with loopback access. Static checks passed. Raw logs and the source input are under ignored `data/portfolio/r02-20261014/`.

Race checks were not run: the current Windows Go environment has CGO disabled and no detected C compiler. Full browser, image, restore and release qualification remain R10/R13 work. The normal five services are healthy on their existing images. Their observed counts remain eight accounts, five transfers and 20 postings, with 45 migration records.

## Limits and next cutover

1. Complete actor-aware legacy read interfaces, including export and operations callers.
2. Convert the remaining direct transaction, audit and maintenance paths.
3. Give global workers narrowly controlled claim functions. Scope each claimed item before ordinary tenant queries.
4. Add new policy migrations. Do not edit applied migrations. Check owners, views, indirect queries, default grants and role inheritance.
5. Run fresh-install and populated-upgrade tests with actual workload logins. Reject incompatible old binaries at cutover.
6. Repeat the integration, fault and capability checks. Record exact source and migration identities.

No production row policy or privilege migration is delivered by this checkpoint. Test-only policies are removed by cleanup. Do not enable row security across all tables yet. Legacy access paths still require conversion.

PostgreSQL custom settings are mutable by the trusted workload. This layer protects against a missing query filter and accidental connection reuse. It does not authenticate a fully compromised shared database client. A stronger threat claim requires a separate design.
