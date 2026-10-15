# PR #11: split plan and CI repair evidence

## Purpose

Replace the oversized [PR #11](https://github.com/painjanevivek/LedgerSync/pull/11) with ten smaller pull requests. The user approved this split.

The source is commit `f453431713ce2342423f76be8f6d70d503cc8d75`. The repaired source is `bed7de5`. The current local Unit 3 and Unit 4 work is outside this change. No merge or deployment is authorized by this report.

## Merge order

Review each pull request against its direct base branch. Merge in this order. After each merge, check the next pull request's base and current checks.

| Step | Pull request | Scope | Review files |
| --- | --- | --- | ---: |
| 1 | [#13](https://github.com/painjanevivek/LedgerSync/pull/13) | Worker, response handling, CI, dependency repairs | 39 |
| 2 | [#14](https://github.com/painjanevivek/LedgerSync/pull/14) | Canonical identifiers | 59 |
| 3 | [#15](https://github.com/painjanevivek/LedgerSync/pull/15) | Controlled ledger commands and database grants | 35 |
| 4 | [#16](https://github.com/painjanevivek/LedgerSync/pull/16) | Progressive evidence disclosure | 57 |
| 5 | [#17](https://github.com/painjanevivek/LedgerSync/pull/17) | Bounded serverless worker support | 19 |
| 6 | [#18](https://github.com/painjanevivek/LedgerSync/pull/18) | Revocable browser sessions | 29 |
| 7 | [#19](https://github.com/painjanevivek/LedgerSync/pull/19) | BFF boundaries and disclosure prerequisites | 49 |
| 8 | [#20](https://github.com/painjanevivek/LedgerSync/pull/20) | Shared UI foundations | 60 |
| 9 | [#21](https://github.com/painjanevivek/LedgerSync/pull/21) | Guided workflows | 56 |
| 10 | This documentation change | Study and implementation evidence | 29 |

The review-file limit remains 60. The critical-file limit remains 30. The visual-baseline limit remains 60. Baseline images and their required ledger use the repository's existing counting rules. The UI splits contain 35, 40, and 40 changed baseline files respectively. No policy exception is used.

## Repairs

1. Check database cleanup errors in the session integration fixture.
2. Remove the unused database error wrapper after its callers are removed.
3. Allow long Recovery status labels to wrap at compact widths.
4. Give the Developer journey grid a bounded column. This prevents intermediate UI overflow.
5. Fix the visual fixture date. Keep timers active for polling, focus, and command checks.
6. Inspect the changed Linux images. Retain screenshot tolerances and accessibility checks.
7. Add exact fingerprints for 12 historical secret-scan false positives. Keep broad exclusions prohibited. The total is 24 existing and new fingerprints.
8. Permit the three named generated public provenance files in the fingerprint contract test. Keep all other path restrictions.
9. Update Go to 1.26.9, x/net to 0.60.0, x/text to 0.42.0, and OpenTelemetry to 1.45.0.
10. Update gRPC to 1.83.2. The container gate found CVE-2026-84445 after the earlier gates passed. See the [maintainer advisory](https://github.com/grpc/grpc-go/security/advisories/GHSA-2v4p-qf9q-27wj) and [patched release](https://github.com/grpc/grpc-go/releases/tag/v1.83.2).
11. Update the locked production web packages within their declared ranges: Next 16.4.0, sharp 0.35.5, and source-map-js 1.2.2.
12. Introduce shared UI modules before their consumers. Keep the original shell and 760-pixel active compact boundary in step 8. Activate the guided shell, 1279-pixel boundary, providers, and route consumers together in step 9.

## Local verification

These results apply to the repaired combined source unless a different scope is stated. They do not replace the checks on each pull request's current commit.

| Check | Result |
| --- | --- |
| Go static analysis | Zero issues |
| Go vet and race tests | Pass |
| Critical-path coverage | 68.6%; required minimum 60% |
| Final Go vulnerability scan after the gRPC repair | No vulnerabilities found |
| Production npm audit | Zero findings |
| Frontend lint | Pass |
| Frontend unit tests | 206 pass |
| Guided browser tests | 219 pass; 4 explicit Windows-only visual cases skipped on Linux |
| Foundation browser tests, step 8 | 202 pass; 4 explicit Windows-only visual cases skipped on Linux |
| Build and bundle budgets | Pass |
| Guided performance tests, isolated run | 3 pass |
| Foundation performance tests, isolated run | 3 pass |
| Historical Gitleaks scans | No findings after exact reviewed fingerprints |

One guided performance run measured a 255 ms long task. The limit is 250 ms. The isolated repeat measured 146 ms and passed all three checks. The limit was not increased. Keep the commit-bound CI performance gate active.

Visual image inspection is implementation evidence. It is not independent product, finance, security, or CODEOWNER approval. The review ledger states this limit.

The full development npm audit still includes development-tool findings. The production audit excludes development packages, as required by the existing gate. No forced framework downgrade was used.

## Required review and release checks

1. Require green checks on each pull request's current commit.
2. Obtain independent review. The author must not be the sole approving reviewer.
3. Complete the required finance, security, and CODEOWNER reviews for the changed boundaries.
4. Run the complete real-stack and release evidence gates before deployment.
5. Apply required schema changes before application consumers.
6. Stop if authorization, replay, delivery, or reconciliation checks fail.
7. Revert application changes if needed. Use a reviewed forward repair for schema or grant changes. Do not delete financial records or change posted history.

PR #11 is the source record. Its original failing checks remain attached to its unchanged source commit. Use the replacement pull requests for review and merge.

## Copyable commit messages

### Step 1

```text
fix(ci): restore worker and API response checks

- Worker liveness, committed-response handling, reproducible CI, and current dependency fixes.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 2

```text
feat(identity): canonicalise UUID boundaries

- Canonical UUID validation across the API, BFF, tenant identity, and stored request keys.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 3

```text
feat(ledger): restrict financial commands and database grants

- Semantic ledger checks, controlled financial functions, migrations 34–36, and adversarial role capability tests.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 4

```text
feat(ui): add progressive evidence disclosure

- Progressive record views, grouped navigation, responsive evidence disclosure, reviewed Linux images, and a stable fixture clock.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 5

```text
feat(deploy): add bounded serverless worker support

- Optional worker drain, lease handling, private Redis and serverless deployment configuration.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 6

```text
feat(sessions): add revocable browser sessions

- Opaque session storage, migration 37, UI preferences, generated contracts, checked fixture cleanup, and locked runtime prerequisites.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 7

```text
feat(bff): enforce opaque sessions and bounded reads

- Opaque-session request proxy, fixed-route mutations, rate limits, bounded responses, security tests, and the scoped disclosure preference adapter.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 8

```text
feat(ui): add guided workspace foundations

- Shared UI primitives, layered styles, mode providers, scoped Recovery reflow, and intermediate page compatibility.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 9

```text
feat(onboarding): add guided ledger workflows

- Simple Home and Tasks, guided account and transfer workflows, public entrypoints, and reviewed Linux state images.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```

### Step 10

```text
feat(docs): add implementation and study evidence

- Architecture decisions, study plans, specifications, and implementation evidence from the original PR.
- Preserve authorization, financial evidence, and CI limits.
- Record this step in the ordered replacement for PR #11.
```
