# R13–R17 review and integration order

The full local implementation remains preserved at commit `840d7a7057c6e91d6abd9a5c205d6881092cc254`. Its hosted code, browser, security, container and contract checks passed. PR #28 originally exceeded the review limit: 194 review files against a limit of 60. This record explains the correction.

The change is now divided into five ordered reviews. A later PR uses the previous review branch as its base. This keeps each review small without changing the existing policy or claiming a scope exception.

| Order | Pull request | Scope | Review files | Base |
|---|---|---|---|---|
| 1 | [#29](https://github.com/painjanevivek/LedgerSync/pull/29) | Verified telemetry transport, local CA configuration, command support, bounded Go build workers and preview guards | 13 | main |
| 2 | [#30](https://github.com/painjanevivek/LedgerSync/pull/30) | Controlled statement comparison, reservations, exact balances, expiry, migrations, grants and generated contracts | 58 | PR #29 branch |
| 3 | [#31](https://github.com/painjanevivek/LedgerSync/pull/31) | Operator pages, BFF routes, original-command recovery, exact balance labels, CSV controls and reviewed visuals | 55 | PR #30 branch |
| 4 | [#32](https://github.com/painjanevivek/LedgerSync/pull/32) | Local qualification, restore, collector, scans, SBOM binding and deployment preparation tools | 27 | PR #31 branch |
| 5 | [#28](https://github.com/painjanevivek/LedgerSync/pull/28) | Phase documentation, retained evidence, chart provenance, completion checks and this integration record | 43 | PR #32 branch |

The console review also contains eight image baselines and their approval ledger. The existing policy counts these separately from ordinary review files. The ledger states author technical inspection. It does not claim independent human acceptance. Critical-file counts also remain within the existing limit of 30.

## What was preserved

The split copies the already committed file blobs into dependency order. The original branch and its commits remain. Its final merge records the review branches as ancestors; it does not force-push or replace the original history. Runtime application code and the qualified financial behavior are unchanged by this review repair. The new source changes are branch-specific preview controls and review documentation.

The normal database, original containers, images, named data volumes, backups, recovery refs, stashes and private local evidence remain. Docker stays stopped during this work. The two intermediate Go stages were tested with two workers and no Docker. Each public branch receives its own hosted checks. Earlier local image records keep their original source identities; they are not relabelled as intermediate Git images.

## Integration instructions

1. Review the PRs in the stated order. Check risk, test, compatibility and rollback sections in each body.
2. Wait for that PR's required checks. Do not treat a parent PR pass as a pass for a later head.
3. Before any main merge, review the existing deployment integration. These review branches have automatic previews disabled. Other branches, including main, retain their existing policies. A main merge or production deployment is a separate decision.
4. Merge the first accepted stage only when its release and review conditions are satisfied.
5. Retarget the next dependent PR to main after its parent is integrated. Verify the remaining file count, source diff and checks again. If the repository uses squash merges, GitHub can compare against an older merge base. In that case, merge the new main into the dependent branch, inspect the result, and push normally. Do not force-push away the preserved work.
6. Repeat the check for every stage. Do not merge a later stage directly to main before its dependencies are present.
7. Stop on a financial invariant, tenant/actor, retry, expiry fence, compatibility, scope or CI failure. Retain evidence and repair the source. Do not remove the gate to make a check green.

## Limits

This is a review correction for the locally qualified portfolio project. It does not approve a managed deployment, bank integration, production SLA, signed release or independent human acceptance. P-062 retains the development-only advisory limit. P-082 retains the recorded SQL/test ordering deviation. P-106 retains the unqualified native Windows throttled LCP result.

See [progress](progress.md), [issues](issues.md), [qualification](qualification.md), and the [review split evidence](../release-evidence/portfolio/r17/review-stack.json).

## Ordered main integration — 17 October 2026

The owner authorized main integration in the order #29, #30, #31, #32 and #28. The first four stages passed all nine required checks and are merged into main in that order. Main deployment is disabled in both Vercel roots. Docker remains stopped; no normal migration, seed or financial command ran. Existing data, containers, volumes, images, backups, recovery refs and stashes remain.

| PR | Qualified head | Main merge |
|---|---|---|
| #29 | `85415d5ac4e57d744b8997a221b2186dfb03cf33` | `7d75ee526448a1c5ef363286927503169ceea80a` |
| #30 | `66ae6e3542e9f2b8bd5eccded0a236d151e4e3fe` | `6b8b23bbb8efb15ed156a2b2ec98e5a9cc660059` |
| #31 | `0bc9fe9f09612481e17b78b82f806a1c88b4e9c4` | `20b9d859bcc0138d935f83f066c2d0da9c846ff8` |
| #32 | `1312a04558e1e90ec66f4820ae4e33536f28107e` | `97f04b0cd1298c6f819425d4af470403244e3ce5` |

This record is prepared for the final #28 review. Its integration review has 44 files, including the new integration evidence record, and one critical CI file. Its required checks and merge are the last step; consult the PR merged state for the final commit. The external phase plan and retained private integration journal record the final outcome. The original qualified application source and all historical evidence identities remain. No branch protection, review budget or production acceptance gate was removed.
