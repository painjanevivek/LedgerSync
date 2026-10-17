# Preserving main consolidation candidate — 17 October 2026

## Source and history

The owner requested the latest local work on main and one remaining branch. [PR #33](https://github.com/painjanevivek/LedgerSync/pull/33) passed all nine required checks and merged at `a4b933ff8cf78d9da01156d9181d21c200043330`.

The remaining source is reviewed in [PR #35](https://github.com/painjanevivek/LedgerSync/pull/35), head `820ddd1`. It includes the separately checked [PR #34](https://github.com/painjanevivek/LedgerSync/pull/34), head `1b47e66`. The final candidate joins that source, this record and all published branch heads. Its first parent is the actual protected main. Older trees do not replace the current application files. No force push or main reset is part of this operation.

The initial inventory contains 38 published branches. The final join also retains the three new source-review heads. It includes parent-fork main `f453431713ce2342423f76be8f6d70d503cc8d75`. All initial published heads and current review heads must be ancestors before their branch names are removed. The fork can remain ahead because it contains its own work. The required parent-fork result is zero commits behind.

## Required merge gates

1. Require all nine checks to pass on the exact current heads of PR #34, PR #35 and this final history review. No skipped, earlier or partial result can substitute for a pass.
2. Confirm that protected main has not moved from the candidate's first parent. Confirm that the candidate differs from the source review only by this record.
3. Use the owner's explicit one-time approval to turn off only required linear history. Keep strict status checks, all nine contexts and their app bindings, PR review policy, administrator enforcement, conversation resolution and the force-push prohibition.
4. Merge the exact history-review head through GitHub. Restore the original linear-history rule immediately, including on a failed merge attempt. Compare the full protection response with its saved original.
5. Confirm that the merged main tree equals the tested candidate tree. Confirm that every published head and the parent-fork head are ancestors. Stop cleanup if any identity changed or any check failed.

These are execution conditions. This candidate record does not claim that a pending check or merge has completed. The merged PR and its Actions links provide the final public result. The private transaction journal retains exact requests, checks, results and protection comparisons.

## Preserve and remove branch names

The private archive is `data/portfolio/branch-consolidation-20261017/`. Verified bundles retain all local and fetched refs, including private recovery histories. Use only published heads as ancestry parents. Some recovery names already point to commits in public history; they do not introduce private-only commits. Keep all private-only recovery commits out of the public join. Create and verify a final bundle before removing any branch name. Record each name and expected head. Delete a remote name only when its current head equals its archived, merged head. Delete local recovery names only after the bundle proves they are recoverable.

Preservation checks passed before final cleanup: all 1935 original source paths remain present; their initial bytes remain in the workspace archive; all 122 original private media hashes match; both stashes remain; all 14 older worktrees retain their own commit, detached state and status; all 473 dirty worktree file hashes match. Keep those folders. Removing a branch name does not authorize removing any worktree, stash, database, image, volume, media or backup.

After cleanup, the repository must expose only main locally and on origin. Use the final bundle and saved branch inventory to recover an old branch name if needed. Keep the old named-worktree mapping in the private archive.

## Verification corrections and limits

Actual-database CI exposed test fixture defects: unsupported USD local-account creation, an incorrect assumed transfer-policy key, a USD-default workflow transfer adapter and stale-before-terminal model ordering. The repaired fixture uses supported INR consistently, preserves original limits and requires one updated row for each isolated policy. A closed account remains terminal with an old revision. Failed logs and the failing model regression remain private. Production financial rules are unchanged.

The complete public-history scan used checksum-verified native Gitleaks 8.30.1, disabled Git text conversion and the repository's existing exact historical fingerprints. The draft scan covered 363 commits and found no leaks. Require a complete scan of this final candidate too. No new scanner ignore rule is part of this change.

Local Docker, normal services and deployments remain stopped. Both Vercel roots block main and every consolidation branch. GitHub CI uses disposable hosted runners; it does not operate the normal local database. The normal schema64 database, stored financial records, retained images and volumes are unchanged.

Branch consolidation does not declare R00–R17 fully accepted. The [audit repair register](audit-remediation.md) and [work packets](work-packets.md) retain open full-trace capture, R09 HTTP/BFF coordination, Windows timing, the development-only dependency finding and actual human acceptance. Lord Master is nominated; independent observations are still required. Older evidence keeps its original source and date.
