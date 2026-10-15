# ADR-002: Tenant context and controlled financial writes

Date: 14 October 2026. Updated: 15 October 2026. Status: R02 locally verified; R03 controlled-write cutover in progress.

## Trust model

The authenticated server establishes tenant and actor identity. Browser request fields and diagnostic headers are not authority. The first database isolation layer protects against missing query filters and pooled-session context leaks while trusting the authenticated application workload.

A shared workload that can freely set its tenant context can impersonate another context after full compromise. A custom PostgreSQL setting alone does not defend against that threat. Any stronger claim requires an independently validated binding or tenant-specific credentials and a new threat review.

## R02 design

Use one explicit transaction helper. Establish validated tenant/actor context on the same connection before queries. Scope context to the transaction. Missing context fails closed. Test commit, rollback, cancellation, pool reuse, joins, exports, recovery, and malformed context with real workload logins.

Inventory table owners, `BYPASSRLS`, role inheritance, privileged function owners, views, global job queues, backups, and reconciliation identities. Define policies per table and operation. Restrict whole-table and schema privileges separately. A global queue claim may be narrowly privileged; it cannot expose general financial-table access.

## R03 cutover

Convert callers before removing their grants. Convert provisioning/account lifecycle, transfers, funding, corrections, then operational/audit writes. Keep trusted authorisation, exact amounts, policy, idempotency outcome, journal, postings, balance versions, audit, and required event obligations atomic.

Use controlled function owners that cannot log in. Fix object resolution and function execution grants. Test actual effective role membership and default privileges. Revoke direct protected-table writes only when every required caller has a verified path.

Do not replay migrations or edit applied migrations during R00/R01. Allocate additive migrations when R02 begins. Prevent incompatible old binaries from using the cutover boundary. Prefer a narrow forward repair over restoring broad grants.

## R02 completion

The isolated candidate now enforces 53 tenant table policies. Bounded queue functions separate global discovery from scoped item processing. Verification completion has a narrow atomic function. Fresh and populated upgrades, actual workload roles, unsafe inheritance and forced non-superuser owner behavior passed. See [cutover procedure](../../security/tenant-context-cutover.md) for deployment order and explicit global exceptions. The normal runtime is unchanged. Broad financial DML remains an R03 gap.

## Historical checkpoint

The existing capability matrix remains an open gap. R00/R01 added design and evidence controls. R02 now implements validated scoped transactions, explicit authenticated actors for developer reads, and cleanup before and after pool use. The same transaction is passed to nested evidence queries. A stale session setting regression was reproduced and repaired. See the [adoption record](../../security/tenant-context-adoption.md). Permanent row policies, remaining caller adoption and financial privilege changes are still pending. The normal runtime is not production-qualified by this decision.

See [database capability matrix](../../security/database-capability-matrix.md) and [work packets](../../portfolio/work-packets.md).

## R03 completion

The local candidate through 000060 uses controlled financial and evidence commands, actor binding, fenced worker completion and a separate bounded maintenance role. The core role matrix and all 36 controlled EXECUTE grants pass. Direct protected financial/audit DML and internal trigger invocation are denied. Required metadata exceptions, administrative import, atomic role application and old-binary incompatibility are in [the cutover procedure](../../security/controlled-write-cutover.md). The normal database is unchanged. R13 and managed-environment gates remain separate.
