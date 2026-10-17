# LedgerSync

LedgerSync is an API-first, closed-loop ledger for exact, same-currency transfers between its own accounts. PostgreSQL stores the financial record; the operator console helps teams inspect balances, transfers, approvals, delivery, and exceptions.

**Current boundary:** local-only software on one workstation. Funding records capture customer-authorized evidence of value confirmed outside LedgerSync. They do not prove bank settlement. Internal reconciliation compares posted ledger and active obligations with stored balance evidence. A separate bounded workflow compares a supplied CSV statement with a fixed ledger snapshot. The sample CSV is synthetic; no bank connection or external settlement proof is supplied. LedgerSync does not initiate bank transfers, payouts, cards, or FX.

Start with the [portfolio review guide](docs/portfolio/README.md) for a five-minute review, architecture, measured evidence, and maintainer handover. The [portfolio phase register](docs/portfolio/progress.md) records local restructuring work. The [master delivery register](docs/plans/ledgersync-master-progress.md) and [master plan](docs/plans/ledgersync-master-product-system-and-website-completion-plan.md) retain commercial and managed-environment gates. Neither local qualification nor a design document is production approval.

## What works today

- Exact integer-minor-unit money and immutable double-entry postings for internal transfers.
- Idempotent financial commands, controlled funding and correction records, approvals, audit events, and transactional outbox delivery.
- Authorized account and transfer reads, internal reconciliation, related evidence, and a local operator console.
- PostgreSQL supplies user-visible balances. Redis supports disposable projections, delivery, and live signals.

Posting and downstream delivery are separate outcomes. An unavailable read is not an empty account, a zero balance, or a passing reconciliation result.

## Current work

1. **Inspect the qualified core.** R13 qualifies the integrated local candidate, including controlled financial writers, restricted workload roles, tenant and actor policies, exact retry, and recovery. The [capability matrix](docs/security/database-capability-matrix.md) states the shared-workload trust limit. The [author decision](docs/release-evidence/portfolio/r13/engineering-decision.md) states the exact source and permitted claims.
2. **Review the verified extensions.** R15 supplies one documented synthetic CSV statement workflow. R16 supplies full-capture internal reservations, full release, fenced expiry and a separate obligation export. Their local financial, authority, model, recovery, API and UI evidence is in the [phase register](docs/portfolio/progress.md). Use a schema66 candidate for these features.
3. **Prepare deployment locally.** R17 prepares a production deployment package. Actual managed identity, backups, failover, operating ownership and deployment remain separate gates. No cloud execution is part of this work.

Provider-led payouts, cards, FX, multiple providers, and automated financial adjustments are deferred. Posted ledger is opening plus immutable postings. Reserved is the sum of active holds. Available is posted ledger minus reserved. A deadline alone does not release a hold. The preserved normal stack stays on schema64 and its original images; use the isolated candidate to review schema66.

## Run locally

From the repository root in PowerShell:

```powershell
.\scripts\doctor-local.ps1
.\scripts\start-local.ps1
Start-Process http://127.0.0.1:3000
```

Use **Guide** in the console for the account → funding → review → transfer → internal reconciliation path. Normal stop preserves data:

```powershell
.\scripts\status-local.ps1
.\scripts\stop-local.ps1
```

The [local runtime runbook](docs/runbooks/local-runtime-smoke.md) covers startup failures, backups, restore, and the separate destructive reset command.

## Reference

- [System design, user sequences, and deployment diagrams](docs/architecture/ledgersync-system-design.md)
- [Rendered diagram gallery: SVG, PNG, and source](docs/architecture/diagrams/index.html)
- [TLS repair and real user journey verification](docs/design/journey-verification-20261010.md)
- [Full user walk-through and latest gap repairs](docs/design/user-journey-walkthrough-20261010.md)
- [Cross-device transfer recovery and verification](docs/design/cross-device-transfer-recovery-20261010.md)
- [UI changes, system rules, and verification](docs/design/ui-system-alignment.md)
- [Owner answers and open decisions](docs/plans/phase-0-owner-answers.md)
- [Architecture and trust boundaries](docs/architecture.md)
- [Private API and retry contract](docs/api-guide.md)
- [Operator and incident guide](docs/operations.md)
- [Database capability matrix](docs/security/database-capability-matrix.md)
- [Release evidence](docs/release-evidence/)
- [Design system](DESIGN.md)

Licensed under the [Apache License 2.0](LICENSE).


## Trusted backend example

Follow [the TypeScript integration guide](examples/typescript/README.md) for one complete synthetic local workflow. Run `node scripts/portfolio/run-developer.mjs` from the repository root to qualify it on disposable targets. The normal local stack supplies dependency image identities only. Browser sessions, own-principal backend credentials, and delegated actor assertions have separate rules. See [phase progress](docs/portfolio/progress.md) for the verified scope and remaining work.
