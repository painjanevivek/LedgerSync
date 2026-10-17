# External statement comparison — CSV v1

Status: R15 verified locally. See the [completion record](progress.md#r15-completion--17-october-2026) for exact evidence. The synthetic schema66 candidate passed the complete statement browser workflow, restricted-role checks, source-byte recovery and financial-preservation checks. The normal schema64 database was not migrated.

## Purpose and authority

Compare supplied external evidence with a saved internal ledger snapshot. Review a difference. Keep the original file and each decision. This module cannot post a journal, change a transfer, or change a balance. A match does not prove bank settlement. The supplied example is synthetic. It is not a bank statement.

Use **Reconciliation → Compare external statement**. Read access requires `reconciliation:read`. Commands also require `reconciliation:write`. Downloads require `exports:read`. The server derives tenant and actor from the authenticated session. The browser cannot select another tenant or actor in a request body.

## File format

Use an unquoted UTF-8 CSV file. Use LF or CRLF line endings. Do not include a byte-order mark, quotes, blank records, NUL characters, unknown columns or reordered columns. Use this exact header:

```csv
row_id,source_account_ref,period_start,period_end,posted_at,external_reference,direction,amount_minor,currency,opening_minor,closing_minor
```

| Column | Rule | Meaning |
|---|---|---|
| `row_id` | Unique stable ASCII token, 1–128 characters | Identifies the source row. Do not reuse it in another file for the same mapping. |
| `source_account_ref` | ASCII token, 1–128 characters; same on every row | Must agree with the fixed mapping. |
| `period_start` | UTC timestamp, whole seconds | Inclusive start. Use `2026-10-01T00:00:00Z`. Years start at 0001. |
| `period_end` | UTC timestamp, whole seconds; later than start | Exclusive end. |
| `posted_at` | UTC timestamp within the period | Source record time. |
| `external_reference` | ASCII token, 1–128 characters | Compare with the internal transfer or funding identifier. |
| `direction` | `debit` or `credit` | Debit subtracts from the source account. Credit adds to it. |
| `amount_minor` | Positive base-10 integer, at most 9223372036854775807 | Exact minor units. For INR, 100 means INR 1.00. Do not use decimals, signs or leading zeros. |
| `currency` | `INR` | The selected pilot currency. |
| `opening_minor` | Signed base-10 int64; same on every row | Balance at the start. Zero is `0`. |
| `closing_minor` | Signed base-10 int64; same on every row | Must equal opening plus credits minus debits. |

Tokens start with a letter or digit. Remaining characters can be letters, digits, dots, underscores, colons, slashes or dashes. The whole file must use the same source, period, currency and opening/closing balances. Totals use wide integer arithmetic. Repeated amounts are valid when row identifiers differ.

The complete file is limited to **1 MiB and 1,000 rows**. Validation is atomic. A bad row rejects the complete import. A preview does not accept a file. Review the preview, then accept the immutable source. See the [synthetic example](../../web/public/samples/statement-v1.csv).

## Mapping and storage

Version 1 permits one immutable INR account mapping per tenant. The account must belong to that tenant. Check it before confirmation. A change or delete operation is not available. At most 100 imports are retained per tenant. This bound is a capacity limit, not a retention schedule. No automatic deletion runs. A later retention or mapping-change design must preserve existing evidence.

The database stores the exact original bytes and their SHA-256. It also stores normalized rows. Identical bytes return the existing import. Changed bytes with a reused row identifier are rejected. The download route requires authorization and a successful audit write. It checks the saved hash before it returns bytes.

## Fixed comparison

A run saves the mapping version, rule version, database cutoff, source period and at most 2,000 eligible ledger items. The saved items include posting identifiers and values. A posting created after the cutoff cannot enter that run, even when its business time is earlier. A prepared run can resume after interruption. A completed run is immutable.

The exact rule compares reference, amount, direction and currency. Automatic matching requires exactly one candidate on both sides. Shared candidates remain ambiguous. A mapping can consume each posting and each source row only once across runs. A concurrent run cannot reuse a consumed posting. Results distinguish unmatched source rows, unmatched ledger items, amount/direction differences, ambiguous evidence and evidence consumed by another run.

## Review and recovery

A review has a version. Read the current version before a decision. Concurrent stale decisions return a conflict. Each accepted decision adds an immutable event. The original classification remains intact. An assignee must be an authorized tenant actor. A comment is limited to 512 UTF-8 bytes; an internal evidence reference is limited to 256 bytes. Neither field is a file upload or an external URL fetch.

A manual link can override the reference after review. It cannot override amount, currency, direction, snapshot membership or prior consumption. A reviewed exception remains an unlinked difference. A review never repairs money automatically.

Each command has an original tenant, actor, operation, payload and idempotency key. Keep that full intent after a timeout. Retry the original command. Do not create a new key to resolve an unknown outcome. Browser recovery stores the digest of an import, not its raw CSV. After reload, reselect the exact file. A different digest is rejected. Unknown or damaged recovery formats stay preserved and block new writes. A known rejection can be closed explicitly after review.

## Downloads and limits

The comparison export is bounded to 4 MiB. It includes saved comparison values and current review evidence. A prepared run has no completed comparison export. Text cells that could execute as spreadsheet formulas are neutralized. Numeric minor-unit fields remain exact decimal strings in the CSV bytes. Import these columns as text in spreadsheet tools to retain values above spreadsheet numeric precision. The original protected source download returns the exact original bytes; treat that source as untrusted evidence.

This release does not parse PDF, OFX, MT940, quoted bank CSV variants or multiple currencies. It does not submit settlement instructions, change money, infer missing bank activity, or establish production reconciliation coverage. These are explicit scope limits.
