# Release and Review Governance

LedgerSync financial, identity, migration, recovery, and release changes require independently reviewable evidence. Repository workflows are necessary but do not prevent unsafe merges unless GitHub rulesets require them.

## Protected-branch contract

The `main` ruleset must:

- require pull requests and disallow direct pushes;
- require at least one approval and a CODEOWNER review;
- dismiss stale approvals when the diff changes;
- require all conversations to be resolved;
- require `Governance gates / Review policy`, `Quality gates`, `Contract validation`, `Production-path CI`, and `Supply-chain and security gates` conclusions applicable to the changed paths;
- prevent force pushes and branch deletion;
- restrict bypass to named emergency administrators, with every bypass linked to an incident and reviewed afterward.

CODEOWNERS establishes routing, not independence. Before production qualification, add at least one second human or GitHub team with the relevant ledger, database, identity, security, or release expertise; the author must not be the sole approver.

## Reviewable-change policy

The governance workflow rejects pull requests above 60 changed files and critical-boundary changes above 30 files. These are safety ceilings, not targets. Financial behavior, grants, migrations, public contracts, and worker ownership should normally be smaller.

If a coordinated generated or mechanical change cannot fit the ceiling:

1. land the behavior and tests separately;
2. attach a file inventory and generation proof;
3. obtain release-engineering and CODEOWNER approval for a temporary threshold change;
4. restore the threshold in the same governance-only pull request.

Do not hide implementation work in an exception or mix database authority, frontend redesign, and worker topology.

## Required evidence

Every critical pull request records:

- the financial/security invariant and credible failure scenario;
- tests written first and commands actually executed;
- compatibility for old/new application and schema versions;
- observability, authorization, tenant, and workload-role effects;
- rollout order, stop conditions, rollback or forward repair;
- required reconciliation, backup, and recovery evidence;
- an independent reviewer with the applicable expertise.

## Last-known releasable commit

A commit is releasable only when its exact SHA has all required checks, schema and grant hashes, SBOM/provenance, reconciliation evidence, recovery evidence, external environment approvals, and no expired exception. Documentation-only status, a green parent, or a manually supplied release-evidence input is not a substitute.

## Emergency bypass

An emergency bypass is time-bounded and must record incident ID, actor, reason, exact SHA, skipped controls, compensating validation, and expiry. It cannot authorize destructive ledger edits, broad standing database grants, or silent reconciliation suppression. The next normal change must restore every bypassed control and attach a post-incident review.
