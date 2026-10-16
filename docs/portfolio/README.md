# LedgerSync portfolio

LedgerSync records internal transfers with exact money. It keeps the financial result separate from delivery, cache freshness, and the browser response. PostgreSQL holds the financial record. The web console helps an operator understand that record.

This is a local engineering portfolio. It is not a bank connection or a production finance service. Funding records contain evidence supplied by an operator. They do not establish external settlement. The [CSV statement workflow](statement-reconciliation.md) compares supplied evidence without changing money. [Internal reservations](reservations.md) separate posted ledger, reserved and available amounts. Their completion records establish the exact local scope; the preserved normal stack remains on schema64. Use a fresh schema66 candidate for extension demonstrations.

Start with the [quickstart](quickstart.md). Use the [five-minute review](demo-guide.md) to inspect the product. Read the [case study](case-study.md) for the decisions and measured limits. The [architecture](architecture.md) shows the trust boundaries and four failure-sensitive sequences. The [maintainer guide](maintainer-guide.md) explains safe changes. Read the [limitations](limitations.md) before repeating a safety or performance claim.

## Evidence map

| Question | Evidence |
|---|---|
| Can a lost response cause a second transfer? | [Retry contract](../../examples/typescript/README.md), [real-stack test](../../web/tests/system/transfer-recovery.real-stack.spec.ts), [candidate qualification](qualification.md) |
| Can the API edit financial tables directly? | [Database capability matrix](../security/database-capability-matrix.md), [actual-role tests](../../tests/integration/database_role_capabilities_test.go) |
| Does a worker repeat work safely? | [Failure demonstrations](failure-demonstration.md), [scheduler fencing test](../../tests/integration/scheduler_fencing_test.go) |
| Does the result agree with an independent model? | [Financial model](financial-model.md), [R08 completion](../release-evidence/portfolio/r08/completion.json) |
| What load was measured? | [Capacity study](capacity.md), [R09 completion](../release-evidence/portfolio/r09/completion.json) |
| Can a candidate be restored? | [Recovery study](recovery.md), [R10 completion](../release-evidence/portfolio/r10/completion.json) |
| Which features are enabled? | [Capability sets](capability-set.md), [phase progress](progress.md) |

Every result has a source identity and scope. A passing historical measurement does not qualify a changed binary. Private raw bundles, databases, credentials, and backup contents stay outside this documentation.
