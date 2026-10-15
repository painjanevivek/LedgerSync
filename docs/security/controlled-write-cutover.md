# Controlled write cutover

The R03 candidate uses migrations 000047–000060 after the R02 tenant migrations. Run this procedure only on an identified target. The normal local database still uses its preserved images and migration history. This document does not claim that it has been cut over.

## Before the change

Preserve the database, source, images, configuration and role assignments. Validate the backup and complete the isolated restore gate. Identify each client that uses this database. Include API, workers, scheduler, reconciliation, provisioning, retention, scripts and support tools.

Stop every old client before applying the candidate. Old binaries use direct writes that the new grants deny. A mixture of old and new binaries is unsupported. A healthy endpoint does not prove command compatibility.

## Apply and verify

1. Apply the additive migrations in order. Do not edit or replay an applied migration. The migrations add functions and access controls. They do not rewrite existing financial rows.
2. Apply the role file with `psql --single-transaction -v ON_ERROR_STOP=1 -f deploy/postgres/roles.sql`. Use `-f -` when the file arrives on standard input. The complete role change must commit together. A failed statement must roll back the role script. Go callers must execute the script in one transaction.
3. Verify safe role flags and inheritance. Function owners cannot log in and cannot have members. Workloads cannot assume an owner role. No workload may create schema objects or truncate a protected table.
4. Verify all 53 forced tenant policies. Check missing context, permitted same-tenant reads, and denied foreign-tenant reads with actual workload logins.
5. Run the core table matrix and complete controlled-function EXECUTE allowlist. The capability matrix version is `2026-10-15.r03.v1`. No `hardening_gap` or unexpected drift is acceptable.
6. Start only matching candidate clients. Use the API role for commands, worker role for queue work, reconciliation role for scheduled comparison, and provisioning role for administrative onboarding.
7. Give retention its separate `ledgersync_maintenance` login binding. This role can call only the bounded retention capability. It has no direct table read or write authority. The production package must provision its secret outside source files.
8. Verify each permitted workflow and its denial. Include transfer replay, account closure, independent approval, expired requests, correction step-up, stale worker claims, recovery replay and reconciliation audit failure.
9. Preserve the test logs, source digest, migration list, role inventory and capability report before removing any isolated target.

## Controlled paths

Account, transfer, funding, correction, provisioning, audit, reconciliation, worker and recovery functions use fixed signatures and fixed object resolution. Financial command functions bind their declared actor to the transaction actor. Required audit and event writes remain in the same transaction as their mutation. Account and reconciliation orchestration retain their command outcome transaction in Go.

Funding request approvals expire after their original request window. Posting also requires a live approved record. An expired request remains visible and can be rejected. The original record is retained. Recovery approval expires after 24 hours. Recovery requires a separate operator. A delivery replay retains its request key and approval identity.

Workers can discover bounded queue batches, then act on one live owner/generation/expiry tuple. A different attempt number, stale generation or expired lease cannot append successful delivery evidence. Manual delivery evidence import is administrative only. No ordinary workload can execute that import function.

Retention deletes only eligible operational rows. It preserves financial records, command outcomes, dead work, replay evidence, delivery references, publication attempts and message receipts. A retained reference makes an event ineligible for cleanup. Cleanup does not remove that reference to force eligibility.

## Remaining direct metadata authority

The source writer inventory names each remaining direct adapter write. These are explicit operational exceptions: account/reconciliation command reservations and retained outcomes; reconciliation command leases; account-created outbox append inside the parent command transaction; onboarding preferences; rate windows; assertion replay hashes; developer credential/webhook metadata and their command outcomes; saved investigation, live room and workspace state. Their tenant policies and existing role checks remain active. They cannot directly change accounts, openings, projections, transfers, journals, postings, funding, corrections, approval evidence or audit rows. R11 separately qualifies optional capability combinations.

The old SQL query template contained unused direct writers and unfenced claims. The candidate replaces them with controlled calls. No generated Go query package was present. PostgreSQL prepares each replacement template in the integration suite. The `sqlc` generator is not installed on this host, so no generator run is claimed.

## Failure and repair

Keep incompatible clients stopped. Save the failed result. Diagnose in the disposable stack. Apply a reviewed forward fix, repeat the affected tests, then repeat the required qualification. Do not restore broad grants or disable policies as an automatic repair. Restore a backup only into an explicitly verified isolated target during the drill.

The workload settings remain a trusted application boundary. A fully compromised shared database credential can choose another setting. These controls prevent omitted filters, pooled scope leaks and arbitrary ordinary financial table writes. They do not independently authenticate a compromised workload.
