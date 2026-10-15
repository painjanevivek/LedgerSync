# Portfolio restructuring baseline — R00

Review date: 14 October 2026, Asia/Calcutta. UTC artifact times can show 13 October. Objective: a strong technical portfolio. This baseline preserves the earlier commercial plan and its open release gates.

## Exact input

The input branch is `main`. HEAD is `ed1662ebc52fbff49bd75bc957b5d2fa637fa73c`. The input was already dirty. The capture recorded 1,565 non-ignored file entries and 608 Git status entries. HEAD alone does not describe this input.

The [source manifest](../release-evidence/portfolio/r00/manifest.json) records file hashes, Git index identity, status entries, and protected configuration metadata. Its file-list digest is `51ec5b242d8577de3c58457fe2adca62eef22f4832bb3ad9712b64a9a3eaf24f`. Secret values were not read into the manifest. File hashes identify content; they do not prove review or production readiness.

Private runtime evidence is in `data/portfolio/r00-20261014/`. That ignored folder contains source-before, container inventories, financial fingerprints, test logs, and a backup. It also contains original copies of the master register, quality workflow, and external portfolio plan. Do not publish database dumps or runtime configuration with portfolio documentation.

## Runtime and backup

Docker Desktop was stopped at entry. The read-only doctor could not reach the engine. Docker Desktop and the existing PostgreSQL, Redis, API, worker, and web containers were started. No normal-stack image was rebuilt. No migration, seed, or financial command was run for these phases. The existing migration and seed containers remained completed.

The private container inventory records seven original container IDs, immutable local image IDs, port bindings, and mounts. These are the existing images. Source tests and runtime smoke therefore qualify different identified inputs. The UI is available at `http://localhost:3000`.

The protected backup is `data/portfolio/r00-20261014/backups/backup-20261013T184305Z-ed1662e`. The existing backup script reported `BACKUP=PASS`. The dump is 371,412 bytes. Its SHA-256 is `d852231ace00e3ab952a323b677d481fe4bd71f9caa7f8c9184dc1d77a87f7c4`. A separate backup root prevents retention rules from removing older user backups.

The database reports 45 migration records. Its latest migration is `000043_reconciliation_scheduler.up.sql`. There are 8 accounts, 5 transfers, and 20 ledger postings. Before/after checks compare these counts and balance, transfer, and posting fingerprints. This detects drift in those measured financial records. It does not claim that every operational table is unchanged while workers run. Backup validation is not a restore drill; R10 owns that proof.

## Representative verification

| Check | Scope | Result and limit |
|---|---|---|
| Go baseline | Domain, transfers, accounts, idempotency, unit, contract packages | Passed. Not the full integration or race suite. |
| Frontend baseline | Financial display, transfer read states, recovery posture, developer contract | 23 passed. No skipped or failed test. |
| Existing-image browser smoke | 14 authenticated main routes | Passed. All returned 200, had a sign-out control and one main heading. No mutating request was observed. This is not a complete user journey test. |
| Backup procedure | Existing database, new backup root | Passed bundle checks and digest capture. Restore not run. |
| Package discovery | `go list ./internal/... ./cmd/...` | Resolved package graph. No import cycle reported. A nonfatal sandbox module-cache write warning occurred. |
| Final runtime/preservation | Doctor, status, health, financial fingerprints, original file hashes | See [progress](progress.md) and private verification records for final results. |

The browser routes were `/`, `/accounts`, `/funding`, `/transfers`, `/approvals`, `/corrections`, `/reconciliation`, `/events`, `/webhooks`, `/developer`, `/recovery`, `/local-status`, `/search`, and `/guide`.

## Reconciled status

M07 wording names OpenAPI 1.14 and unfinished developer capabilities. Current source declares OpenAPI 3.5.0 and contains provisioning, shared assertion replay protection, and webhook verification paths. M08 still asks to build Approvals and Webhooks, but their current routes and feature code exist. M09 asks for a style split, but foundations, layout, primitives, tokens, and feature styles already exist. These are source corrections, not proof that every acceptance criterion has passed.

M11 remains open. Existing controlled transfer functions do not mean that every financial writer uses a controlled boundary. Broad workload grants and database tenant isolation still need R02/R03. Current source includes prior distributed-system study work. Preserve it and qualify it through the later Systems Lab phases.

The earlier external pilot plan is retained at `D:/Work/Project/Plans/LedgerSync/ledgersync-trust-first-product-redesign-and-pilot-plan.md`. The portfolio plan is `D:/Work/Project/Plans/LedgerSync/ledgersync-portfolio-restructuring-plan-2026-10-13.md`. No older plan is replaced by a production-completion claim.

## Safe test environment

Use the existing acceptance tooling for later destructive integration and fault tests. It must create a separate project with the form `ledgersync-acceptance-<14 digits>-<8 hex digits>`. Confirm labels, data-volume ownership, ports, images, and fixture identity before a mutation. Never target `compose` or reuse the normal database volume. Preserve failure artifacts before cleanup. Remove only a verified disposable target.

R00/R01 do not need a new populated stack: their added code is an import guard and an offline contract validator. Baseline UI reads used the normal runtime without submitting financial commands. R02 must create and verify the disposable database before it changes roles, policies, or migration behaviour.

## Index deviation

Automatic approval review rejected external repository indexing because it could export private source to an unverified destination. A local [static inventory](source-inventory.json) substitutes for this phase. It covers 49 production Go package directories, 85 route registration lines, 90 web page/route files, 65 table declarations, 429 named Go test/fuzz declarations, 358 optional-feature references, and 110 resource-bound declarations.

These are declaration counts, not counts of distinct enabled capabilities. SQL history can create the same table more than once. Lexical references are not a complete call graph. Go package discovery supplies a separate cycle check. The external index was not refreshed. This limitation remains recorded as P-003; it does not block local implementation.
