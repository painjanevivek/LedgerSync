# Solo-maintainer review and the October consolidation

The repository owner, Vivek Painjane (`painjanevivek`), works alone. On 15 October 2026, the owner approved merging without another reviewer, keeping all required CI checks, and consolidating all branches with verified local work.

A critical pull request by this maintainer may truthfully select `Solo-maintainer review applies under repository policy`. This records the approved exception; it does not claim an independent or CODEOWNER review. Other authors retain the independent-review declaration.

The normal limits remain 60 review files, 30 critical files, and 60 visual baselines. One consolidation exception applies only to PR #23 from `codex/portfolio-restructuring-r00-r10`, by the same maintainer in this repository. It expires on 31 October 2026. Every changed path must appear in the checked-in allowlist. Bounds are 1200 review files, 400 critical files, and 120 visual baselines. The PR must explicitly select `Consolidation scope exception 2026-10-ledgersync applies`. Visual changes still require the approval ledger. The exception cannot modify its own guard or approval document.

CI reads this approval from the pull request's trusted base commit, never from an approval first introduced in that pull request. This dedicated governance change must merge before the consolidation can use it. All nine required CI checks, strict base freshness, conversation resolution, administrator enforcement, linear history, and no-force-push protections remain enabled.

Verify the guard with `pwsh -NoProfile -File tests/scripts/test-pr-governance.ps1`. Its negative cases cover author/repository/branch/PR mismatch, missing acknowledgement, expired or excessive scope, unlisted paths, and self-approval attempts. This exception does not establish production readiness or complete unfinished portfolio phases.
