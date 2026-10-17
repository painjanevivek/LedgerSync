# Maintainer guide

## Find the owner

Use the [module map](module-map.md) and [ADR 001](../architecture/decisions/001-portfolio-module-boundaries.md). Application modules own use cases and interfaces. PostgreSQL, HTTP, telemetry, and network adapters belong to platform or transport packages. Process startup belongs to `cmd`. The web feature owns its interaction state. Do not add a second financial authority to simplify a screen.

## Change a financial rule

Write the failing financial tests first. State the amount, currency, authority, replay, version, audit, and event obligations. Test the actual restricted database login. Preserve immutable posted evidence. Add the next ordered migration; never change an applied migration. Inspect the controlled-function owner, fixed search path, grants, forced row policies, and direct-write denials. Extend the independent model when balance meanings change.

Use forward repair for financial schemas. An older binary is compatible only when its assumptions remain true. A rollback must not discard records, active obligations, or original command identities. Check the [database capability matrix](../security/database-capability-matrix.md) after every authority change.

## Select checks

Run the affected unit, contract, architecture, and adapter checks first. Use the existing CI workflows as the full committed-release authority. The [qualification guide](qualification.md) lists isolated integration, failure, browser, model, optional, and recovery commands. Run sustained capacity alone. Keep source unchanged during a guarded run. Preserve a failed result and create a new run after repair.

After a schema or financial change, repeat fresh migration, supported populated upgrade, actual-role denials, independent model, replay, worker recovery, and backup restore. Do not replace these checks with UI snapshots. Next.js changes must follow the local version documentation named in `web/AGENTS.md`.

## Update dependencies and contracts

Use pinned tool versions from the workflows and lockfiles. Review advisory reachability and runtime inclusion. Keep production and development findings separate. Regenerate developer artifacts with the repository generator and run its drift check. Run Go vulnerability analysis, the production npm audit, source/history secret checks, image scans, and SBOM generation for the final candidate. Do not add a broad ignore rule to obtain a pass.

Run `node scripts/portfolio/verify-portfolio-docs.mjs` after documentation changes. It checks local links, command entry files, the source names in the diagrams, and chart values against the archived R09 report. To render the chart again, run `python scripts/portfolio/render-capacity.py` in a separate plotting environment with matplotlib 3.10.7. The chart reads its public JSON source and writes a public SVG plus a private preview. Plotting tools are not application dependencies. Inspect the preview before accepting a changed figure.

## Operate locally

Use the [runtime runbook](../runbooks/local-runtime-smoke.md) for status, backup, restore, and stop. Verify backup metadata and digest before a restore. Cluster policy-role identities must exist as inert roles before `pg_restore`; use the checked helper. Do not inherit login credentials or memberships into those roles. Reapply the separate workload credentials after restore and verify direct-write denials before serving traffic.

If a command result is unknown, retain its original intent, reference, and key. Read or replay that command. Do not create an automatic replacement. If delivery fails, inspect the event and fenced attempt independently of the financial receipt. If a balance read fails, show unavailable and preserve known evidence.

Keep private bundles under ignored `data/portfolio` and local backups under `data/local-backups`. Stop safely with `scripts/stop-local.ps1`. Remove a failed temporary target only after checking its exact ID, labels, image, mounts, and ownership. Keep normal named volumes. No cloud execution belongs to this local handover.

## Maintain active obligations

Keep posted ledger, reserved and available values separate in reads, receipts, events and exports. Check the exact equation before showing a split receipt. Missing historical fields stay unconfirmed. A hold remains active after its deadline until one capture, release or expiry transaction commits.

The worker has five queues. A health check must not report success after only four. Rehearse dependency loss and graceful termination on exact disposable identities. Restore statement source bytes and snapshots with hold receipts, events and expiry leases. A restored stale lease must be fenced before a renewed worker can complete it. Keep schema66-compatible images for rollback; do not erase obligations or version-2 events.
