# Tenant context cutover

Status: R02 verified in the isolated local database on 15 October 2026. This is a deployment procedure. It does not authorize a normal database migration.

## Required order

1. Preserve the database, current images, configuration, source identity, and migration history. Validate the backup. Use the restore procedure before a release.
2. Stop API, worker, scheduler, reconciliation, provisioning, retention, and support clients that use the old database contract. Do not use a rolling mix of old and new binaries.
3. Apply additive migrations `000044`, `000045`, and `000046`. Do not edit an applied migration. No financial row is rewritten by these migrations.
4. Apply `deploy/postgres/roles.sql` as the database administrator. This assigns fixed function owners and execution grants. The script rejects unsafe role flags and inherited roles.
5. Verify that all 53 tenant-bearing or tenant-related tables have enabled and forced row security. Verify that queue and preflight owners cannot log in, have no members, and do not bypass row security.
6. Start only the matching source candidate. The API requires the new currency preflight function. Tenant operations must install tenant and actor context on their own transaction. Old unscoped reads return no tenant rows. Old writes fail. These are rejection controls, not a supported compatibility mode.
7. Run actual workload-role checks. Verify a permitted read and command as well as each denial. A successful health response alone is insufficient.
8. Keep the previous binary stopped if the new candidate fails. Diagnose in the isolated stack. Use a reviewed forward migration. Do not disable row security or restore broad roles as an automatic rollback.

The local normal stack was not cut over. It retains its existing images and 45 migration records. R13 will qualify a complete candidate. R17 will rehearse the deployment package locally.

## Authority exceptions

| Path | Authority and limit |
|---|---|
| Queue discovery | Three bounded claim functions return at most 100 eligible jobs with a lease of at most ten minutes. The non-login queue owner has operational table grants. It has no financial table grants. |
| Item processing | A narrow function resolves one live owner/generation/expiry tuple. The item transaction installs the resolved tenant and worker identity. Every state change checks the fence again. |
| Webhook activation | One function updates a pending endpoint and appends its verified event in the same transaction. It requires a live claim and matching transaction scope. The worker has no direct endpoint update grant. |
| Currency startup check | A non-login preflight owner can read four tables through explicit policies. Its only workload entry point returns one boolean currency-invariant result. It returns no tenant rows or counts. |
| Scheduler | Existing controlled functions bind permitted tenants to the authenticated database login. Existing barrier views apply that mapping. The scheduler still cannot read base financial tables directly. |
| Reconciliation, retention, cache rebuild | Explicit tenant scope and named system actors. These commands must receive an authorized tenant from their local operating boundary. |
| Assertion replay store | Global hashes and expiry times only. The store contains no tenant financial record. Cleanup remains bounded. |
| Node clocks and schema metadata | Global infrastructure state. Clock counters have no financial authority. |
| Semantic diagnostic view | Administrator-only diagnostic view. No workload read grant is added. |
| Administrator, restore, migration | PostgreSQL superuser authority is outside workload isolation. Keep these credentials out of applications. A restore can use `row_security=off`; close the restore connection before workload checks. |

Custom settings are mutable by the trusted workload. R02 prevents missing-filter and connection-reuse errors. It does not authenticate an attacker who has full control of a shared workload credential. R03 separately restricts direct financial writes.
