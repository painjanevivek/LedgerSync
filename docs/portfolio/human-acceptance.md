# Fresh-person review and handover

Status: the owner nominated **Lord Master**. Await their observations and confirmation that they are unfamiliar with LedgerSync. This guide does not record a human pass. Use the opaque reference `reviewer-lord-master` for private notes.

Docker and deployments must stay stopped during the current repair work. Read the guides and inspect retained evidence now. Mark every task that needs a running database as **not run**. Do not start a stack, migrate normal data or make a financial command to complete this form.

## Prepare the review

1. Use a reviewer who did not write the feature. Record an opaque reviewer reference and their role. Keep personal contact details out of Git.
2. Record the exact source or source manifest and the environment. Identify a synthetic candidate before any later live task. The normal schema64 data is not a test fixture.
3. Give the reviewer the [overview](README.md), [quickstart](quickstart.md), [review guide](demo-guide.md) and [limits](limitations.md). Do not explain the internal architecture before the first task.
4. Ask the reviewer to record what they did, how long it took, where they needed help and what they expected. Keep the wording of an unclear instruction.
5. Do not record tokens, full financial exports, passwords or personal data. Use synthetic labels and the minimum evidence needed to describe a problem.

## Tasks and success conditions

| Task ID | Ask the reviewer to do this | A passing observation |
|---|---|---|
| purpose | Explain the problem after reading the overview. | They describe an internal ledger and explain that it does not prove bank settlement. |
| boundaries | Identify posted money, delivery status and stale evidence in the written examples. | They explain why delivery or cache failure cannot change posted money, and why an old result is not current health. |
| first-journey | In a later approved disposable candidate, create two accounts and use controlled funding and independent review. | The guide supplies all prerequisites. The reviewer completes the permitted workflow without an undocumented code or configuration edit. |
| transfer-review | Prepare, inspect and confirm a synthetic transfer. | They can state the accounts, exact amount and effect before submission. |
| unknown-result | Use the guided response-loss task in the approved candidate. | They recover the original result with the same key and full intent. They do not create a replacement key to clear uncertainty. |
| investigation | Inspect a record, related evidence, reconciliation and a bounded export. | They distinguish internal reconciliation from supplied external statement evidence and can locate the audit reference. |
| reproduce-failure | Follow the technical guide for one approved failure and inspect its retained result. | They can identify the fault, recovery, source and financial checks. A preview alone is not a passing live reproduction. |
| safe-handover | Find maintenance, backup, upgrade and shutdown instructions. | They can explain the stop procedure and the rule that normal data and volumes must be retained. |

For each task, record `passed`, `failed` or `not_run`; elapsed seconds; help required; the observed result; and any issue. Time is a measurement, not an invented pass threshold. A task that needs undocumented help requires an instruction repair and a repeat.

Ask these questions after the tasks:

- What does PostgreSQL decide? What does Redis decide?
- What should you do if a command times out after it might have committed?
- What remains unproved by the local performance, recovery and accessibility results?

## Record and close

Use [the record template](../../contracts/portfolio/human-acceptance.template.json). Save completed private notes in ignored `data/portfolio/`. The [record checker](../../scripts/portfolio/human-acceptance.mjs) validates structure and task completeness. It cannot authenticate a person or prove independence. An actual reviewer must supply the observations and attestation. A maintainer must check their origin before signing off R05/R14.

Keep failed notes. Fix blocked instructions and repeat only affected tasks plus any required integrated checks. Human assistive-technology review and physical devices have their own acceptance scope. Do not convert browser automation, author review or generated records into independent human approval.
