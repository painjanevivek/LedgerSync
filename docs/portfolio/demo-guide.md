# Review and demonstration guide

## Five-minute review

This path assumes the local stack is already running. Installation time is separate. Reading existing records does not require a new financial command.

1. Open the overview. Identify the account, amount, currency, version, and freshness. Read the [balance rules](user-journey.md). An unavailable read must not appear as zero.
2. Open an existing transfer receipt. Separate the posted or rejected financial result from event delivery and read freshness. The technical details disclose the durable reference and replay evidence.
3. Open the internal reconciliation result. This compares LedgerSync's own evidence. It does not prove that a bank settled funds.
4. Open Recovery. Separate normal backup validity, an actual restore result, and historical synthetic candidate proof. A passing candidate drill cannot make a normal backup current.
5. Read the [capacity result](capacity.md) and [limitations](limitations.md). Ten offered commands per second means 8.8 new postings per second for the selected mix. It is not an HTTP or managed-service rate.

Use the isolated candidate command in the [quickstart](quickstart.md) when a reviewer wants to reproduce writes. It tests account lifecycle, controlled funding, unknown response recovery, exports, evidence handover, and process restart. Read the actual result before saying that this candidate passed.

## Technical review

Read the [architecture and sequences](architecture.md), then inspect these independent local profiles:

```powershell
node scripts/portfolio/run-demo.mjs
node scripts/portfolio/run-model.mjs
node scripts/portfolio/run-recovery.mjs
node scripts/portfolio/run-optional.mjs
node scripts/portfolio/run-developer.mjs
node scripts/portfolio/run-statement.mjs
node scripts/portfolio/run-reservations.mjs
node scripts/portfolio/run-extension-recovery.mjs
```

Run one profile at a time. Keep source unchanged while each runner is active. The default demonstration runs seven fixed failure scenarios. It drives owned test resources; the browser cannot start host faults. The model uses independent exact arithmetic. Recovery restores an unfiltered synthetic backup into a new target before financial validation. Optional profiles cover explicitly supported feature sets. The developer profile tests a separate client and signed receiver.

For a measured capacity repeat, use this command alone, without builds or other database tests:

```powershell
node scripts/portfolio/run-capacity.mjs --batch=10 --rate=10 --steady=300
```

A ten-second calibration cannot qualify sustained capacity. Read each `run-manifest.json`, named scenario result, source identity, cleanup result, and artifact checksum. Use [verify-demo](../../scripts/portfolio/verify-demo.mjs) or [verify-recovery](../../scripts/portfolio/verify-recovery.mjs) for the corresponding bundle. Do not publish raw bundles.

## Selected extension review

Use a freshly qualified schema66 candidate for these writes. The preserved normal stack can still use earlier images and schema64. Do not start normal migration as a demonstration step.

1. Open external statement comparison from Reconciliation. Inspect the fixed account mapping. Validate the documented synthetic CSV, accept its immutable bytes, compare the saved ledger snapshot and review a difference. Download the original source and comparison. A review cannot post a financial adjustment.
2. Open internal reservations from an account. Inspect available funds and posted ledger separately. Reserve a full amount, then capture or release it. Read the current hold separately from its original command receipt. Inspect the obligation CSV and audit reference. Deadline passage alone never releases a hold.
3. Read the [statement rules](statement-reconciliation.md), [reservation rules](reservations.md), [event compatibility](../../contracts/balance-events.md) and [local deployment package](../../deploy/production/README.md). Keep their scope limits with the demonstration.

## Explain one real defect

Use the [session boundary repair](case-study.md#defect-and-repair). Show the exact-digest and actor checks in the regression test. Explain why tenant filtering alone did not protect session lookup. The regression proves the selected negative cases. It is not proof against a fully compromised shared API workload.

## End safely

Each runner checks exact temporary resource identities before cleanup. If cleanup fails, keep the bundle and follow its recorded identities. Never remove a container by a broad name pattern. Use the normal stop command in the quickstart only when the reviewer wants the normal application stopped. No video or public demo deployment is part of this guide.
