# Main branch consolidation — 17 October 2026

The owner requested the latest local work on `main` and one remaining branch. GitHub initially has 38 branches. Main is 20 commits ahead of and 51 behind the parent fork. These counts describe commit ancestry; they do not establish that the latest application code is absent.

## Preserve first

The private checkpoint is `data/portfolio/branch-consolidation-20261017/`. It contains a 1935-file workspace copy, source/index identities, branch heads, worktree and stash records, plus verified Git bundles before and after fetching all published heads. The bundles also retain private recovery branches. Never publish those private recovery histories.

Retain every older worktree and its uncommitted files. A branch may be removed only after its head is retained in the verified archive and its source changes have a disposition. Detach older worktrees at their existing commit before removing their branch names. Do not delete worktree folders, stashes, databases, volumes or media.

## Review the content

The branch comparison checks each changed path against its common base and current main. Most work is either identical or superseded by later qualified code. An unchanged base file with a branch-only change receives a separate review.

| Older branch work | Current disposition |
|---|---|
| Live rooms and compatibility at old `000034`/`000035` | Current migration names are `000037`/`000038`. Preserve the current sequence. |
| Opening import at old `000037` | Current controlled import is `000064`. Do not create a second migration numbered 37. |
| Sessions/preferences at old `000037` | Current sequence is `000062` and `000063`, including forced policies. |
| Old direct-DML revoke and tenant expand/force migrations | Current controlled functions, role script and forced policies supersede the older sequence. Retain the current denial tests and capability inventory. |
| Old tenant-context tests | Current unit and actual-role integration tests cover the stronger scoped and forced-policy contract. Missing-context compatibility tests from the earlier design are obsolete. |
| Root Vercel rejection script | Current roots explicitly identify the Go API and `web` frontend. Preserve their validation and deployment blocks. Do not restore a blanket root rejection that conflicts with the declared backend root. |
| Database capability and tenant rollout runbooks | Restore the intent through current runbooks with corrected migration references and stop conditions. |
| Focused retry availability | Carry forward the branch's accessible disabled/busy explanation and preserve native disabled-button behavior. |
| Disclosure preferences | Use the existing tenant/operator-scoped preference helper. Refuse blocked storage safely. A standalone presentation primitive may have no session; authenticated consumers still use the strict session hook. |

The initial disclosure port exposed a standalone-rendering regression in existing tests. The repair adds an optional context read for presentation only. It does not weaken the required authenticated hook, grant authority or fetch private records. All 256 web unit tests, types and lint passed after this repair. Native browser and hosted results retain their own source identities.

## Publish in reviewable stages

1. [PR #33](https://github.com/painjanevivek/LedgerSync/pull/33): bounded diagnostics, native alert tests, browser report privacy and deployment guards; 27 files. All nine checks passed at `ed7bf85`; main merge `a4b933f` has the same tested tree.
2. [PR #34](https://github.com/painjanevivek/LedgerSync/pull/34): complete fault mapping, independent workflow/history checks and bounded component capacity tools. Actual-database runs exposed inconsistent fixture currencies and an incorrect assumed policy key. The repaired workflow uses INR for local accounts, its independent model, both isolated database policies and its explicitly configured transfer adapter. Policy updates require one affected row. Existing USD model seeds and production rules remain unchanged. Failed logs remain private. Exact-head checks must pass after the repair and the real main-base update.
3. [PR #35](https://github.com/painjanevivek/LedgerSync/pull/35) carries the retained audit records, human-review protocol, current documentation and recovered UI/runbook changes. Its base follows stage 2. Keep each review within the existing scope limit.

All nine required main checks remain mandatory. Before merging each dependent stage, update it to the actual protected base, inspect its remaining diff and require checks on the exact head. Preserve failed attempts. Do not weaken a check or replace a failed result with an earlier pass.

## Fork ancestry decision

Local draft `32aa535` joins all 38 published branch heads while retaining the exact initial main tree. It contains the parent fork's main history and excludes private backup histories. The draft is unpublished. It demonstrates a preserving history join; it is not the final application candidate.

Main currently requires linear history. A squash can carry the code but cannot make the original parent commits ancestors. The owner approved one history merge: allow that merge after all nine required checks pass, then restore the linear-history rule. Apply this exception only to the reviewed history merge. Keep every other required check, force-push prohibition and deployment block. Save and compare the protection settings before and after the merge.

A native Gitleaks 8.30.1 scan of the unpublished public-history draft passed: 363 commits and about 26 MB were scanned. The official binary checksum was verified first. Git text conversion was disabled for the complete scan. An earlier attempt stopped at a local text-conversion error; its partial result is not accepted. The complete scan uses the repository's existing exact historical fingerprints. No new ignore rule was added.

## Current limits

Docker, normal services and deployments remain stopped. Review branches explicitly disable Vercel deployment at both roots. Branch cleanup does not close the [remaining audit gates](audit-remediation.md): R09 HTTP/BFF coordination, full trace/runtime/image qualification, Windows timing, the development-only dependency finding and actual human acceptance remain separate work. Lord Master is nominated; observations and independence confirmation remain open.
