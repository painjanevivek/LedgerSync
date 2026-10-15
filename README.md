# LedgerSync

LedgerSync is an API-first, closed-loop ledger for exact, same-currency transfers between its own accounts. PostgreSQL stores the financial record; the operator console helps teams inspect balances, transfers, approvals, delivery, and exceptions.

**Current boundary:** local-only software on one workstation. Funding records capture customer-authorized evidence of value confirmed outside LedgerSync. They do not prove bank settlement. Reconciliation currently compares stored ledger postings with LedgerSync balance projections; it does not compare them with a bank or payment-provider statement. LedgerSync does not initiate bank transfers, payouts, cards, or FX.

The [master delivery register](docs/plans/ledgersync-master-progress.md) is the source of current status. The [master plan](docs/plans/ledgersync-master-product-system-and-website-completion-plan.md) records the delivery gates. Neither local qualification nor a design document is production approval.

## What works today

- Exact integer-minor-unit money and immutable double-entry postings for internal transfers.
- Idempotent financial commands, controlled funding and correction records, approvals, audit events, and transactional outbox delivery.
- Authorized account and transfer reads, internal reconciliation, related evidence, and a local operator console.
- PostgreSQL supplies user-visible balances. Redis supports disposable projections, delivery, and live signals.

Posting and downstream delivery are separate outcomes. An unavailable read is not an empty account, a zero balance, or a passing reconciliation result.

## Current work

1. **Finish the database boundary.** Complete the controlled-command cutover, remove broad direct financial-table writes from workload roles, and enforce tenant isolation at the database boundary. Current grants and the remaining gap are recorded in the [capability matrix](docs/security/database-capability-matrix.md).
2. **Complete the existing developer and operator workflows.** Keep financial evidence, failure recovery, and authorization clear without adding new product claims.
3. **Validate external reconciliation with a design partner.** If a recurring problem is confirmed, start with one read-only external source and a mapped clearing account: preserve the source and cutoff, match deterministically, and review exceptions. This is a proposed product slice, not a current feature.

Provider-led payouts, cards, FX, multiple providers, and automated financial adjustments are deferred. Pending and available balance semantics will expand only when a real hold or settlement workflow requires them.

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
