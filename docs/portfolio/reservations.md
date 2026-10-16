# Internal reservations

Status: R16 verified locally. See the [completion record](progress.md#r16-completion--17-october-2026) for exact state, race, model, API, browser, export and recovery evidence. No normal database migration or financial command ran. Use a schema66 candidate.

## What the values mean

| Value | Meaning | Example before reservation |
|---|---|---|
| Posted ledger | Opening balance plus immutable journal postings | INR 10.00, or 1000 minor units |
| Reserved | Sum of active internal holds | 0 |
| Available | Posted ledger minus reserved | 1000 |

Reserve 300. The posted ledger stays 1000. Reserved becomes 300. Available becomes 700. The source balance version advances once. No journal is posted.

Release that hold. The posted ledger stays 1000. Reserved becomes 0. Available becomes 1000. The source version advances once. No journal is posted.

Capture that hold instead. The posted ledger becomes 700. Reserved becomes 0. Available stays 700. The destination gains 300. Capture posts one balanced transfer journal. Each affected balance version advances once. A private temporary release inside the transaction is never a separate public version or event.

A second active hold stays reserved during capture of the first. Ordinary transfers and corrections can spend only available funds. Credits increase posted and available by the same amount. They do not release a hold.

## Supported scope

Use one source, one destination and one positive exact int64 minor-unit amount. Both accounts must be customer accounts in the same tenant and currency. The console uses the INR pilot. The caller needs current source debit permission, destination credit permission and transfer-write scope. The server derives identity. The request body cannot select an actor or tenant.

The expiry lifetime is from 1 to 86400 whole seconds. The database measures it at acceptance. An original-key replay returns that original deadline. A replay never extends the lifetime. There is no renewal or edit command. At most 1000 active holds are allowed per tenant. Historical evidence is retained; no automatic deletion runs.

Version 1 supports full capture, full release and worker expiry. It excludes partial capture, external card authorization, FX, external settlement and overdrafts.

## State and original receipts

A hold starts `active`. Exactly one transition can change it to `captured`, `released` or `expired`. The hold version is 1 when active and 2 when terminal. The balance version is a separate value and can advance for other financial commands.

Each command keeps tenant, actor, operation, original key and full logical intent. A lost response requires an original-key retry. The browser saves that complete intent before dispatch. It fails closed when recovery storage is unavailable. A future or damaged recovery format stays preserved and blocks new writes. It does not silently create another command.

A replayed creation receipt still says `active` after a later capture. That receipt describes the original accepted command. Read the current hold separately. The UI labels these two forms of evidence separately.

Capture checks the database deadline and the existing transfer boundary. Current freeze, permission, currency, amount and velocity rules still apply. A business denial leaves the complete hold active and its funds reserved. The denied key keeps the same rejected outcome. Fixing a policy does not change that original result. Read current evidence and make a deliberate new decision if a new attempt is appropriate.

Release can proceed while the account is frozen. Account closure still requires exact zero posted and available values. An active positive hold therefore prevents closure. Historical postings remain immutable.

## Expiry and recovery

Passing a wall-clock deadline does not itself change money. The hold stays reserved until a database expiry transaction commits. Capture is denied at or after the deadline. A delayed worker can reduce availability, but it cannot release funds by assumption.

Only the worker can discover expiry jobs. Discovery is limited to at most 100 due jobs and uses row locking with skip-locked selection. Claims have an owner, generation and database-clock lease. Completion checks all of them again. An old worker generation cannot complete a renewed claim. The expiry transaction updates state, available funds, balance version, audit, durable event and job completion together.

Restore active and terminal holds, command receipts, hold events and expiry jobs with the full database. Do not rebuild obligations from Redis. Restart a compatible worker. Leased work can resume after its retained lease expires. Replaying a terminal command must not post or release funds again.

## Events, comparison and rollback

Historical `account.balance.changed.v1` events imply zero reserved funds. A new event whose posted and available values differ uses `account.balance.changed.v2`. Version 2 carries posted ledger, reserved, available and balance version. The projection consumer validates the exact equation. Redis remains disposable. Customer reads still use PostgreSQL.

Internal reconciliation independently sums active holds. It compares posted ledger with opening plus immutable postings, then compares available with posted minus holds. A reservation is an obligation, not a statement posting. External statement comparison still selects immutable ledger postings only.

The separate reservation CSV uses one authorized read-only repeatable database snapshot. It includes snapshot time, identity, exact amount, active reserved amount, currency, both accounts, state, hold version, expiry and capture reference. Active amounts stay reserved after their deadline until a terminal transaction commits. Terminal rows show zero reserved amount. The export requires both `transfers:read` and `exports:read`, a successful audit write and an audit reference. It refuses more than 2,000 holds or 2 MiB; it never silently truncates a snapshot. Text formulas are neutralized. Import exact minor-unit columns as text in spreadsheet tools. This CSV is not a database backup and does not add holds to the immutable journal export.

New transfer receipts include exact `posted_minor`, `available_minor` and `reserved_minor`. Old receipts remain byte-for-byte historical results. The console shows separate available/reserved receipt values only when those fields are present and the exact equation agrees. Missing values remain unconfirmed. Do not relabel an old receipt as the latest account balance.

A binary that assumes posted equals available cannot safely process active holds or version-2 events. A compatible rollback must retain schema66 reservation and event support. Otherwise pause writes, keep evidence, use a forward repair and repeat reconciliation and original-key checks. Do not drop reservation tables or lower migration history to make an old binary start.

See [ADR 008](../architecture/decisions/008-internal-reservations.md) and the [production package](../../deploy/production/README.md).
