# Unit 3 and Unit 4: final validation

**Review date:** 2026-10-12, Asia/Calcutta.  
**Status:** Verified. All 11 required steps are complete.

Evidence timestamps use UTC. The review date uses the client's time zone.
The [implementation plan](../../plans/ledgersync-unit-3-unit-4-implementation-plan.md)
defines the required scope. The product delivery register remains the release authority.

## Implemented functions

| Function | Implementation | Evidence |
|---|---|---|
| API replicas | Private readiness-aware gateway; bounded connection pools; separate node identities. | [Replica configuration](replica-configuration.md), [API recovery](api-failure.json) |
| Parallel workers | Database claim generations, expiry, renewal, and fenced completion. | [Worker sharing](parallel-workers.json), [crash checks](worker-crash-verified.txt) |
| Logical clocks | Persistent Lamport counters, immutable origin metadata, separate send and receive records. | [Clock rules](clock-rules.md), [causal trace](causal-trace.json) |
| Mutual exclusion | Existing tenant and account locks; bounded conflict handling; shared idempotency. | [Concurrent commands](concurrent-transfers.json) |
| Snapshot evidence | One repeatable-read ledger report; outbox counts in that snapshot; separate service samples. | [Snapshot example](snapshot-report.json) |
| Scheduler takeover | Optional restricted identity; database lease time; increasing generations; unique intervals. | [Scheduler rules](scheduler.md), [takeover](scheduler-takeover.json) |
| Operator UI | Collapsed technical evidence in Developer, event details, and reconciliation details. | [UI review](ui-design-review.md), [real browser journeys](final-financial-verified.txt) |
| Scaling study | Eight equal-method workloads across four replica configurations and two account patterns. | [Performance report](performance-report.md), [recovery timing](recovery-report.json) |

## Checks

| Check | Result | Record |
|---|---|---|
| Go unit, integration, contract, and fault checks | 42 packages passed. 423 top-level tests passed. | [Final Go log](final-go-verified.txt) |
| Go static checks | Passed for application packages and commands. | [Go vet](final-go-vet.txt) |
| Frontend unit checks | 214 passed; none failed. | [Unit log](final-web-unit-verified.txt) |
| Frontend lint and types | Passed. | [Lint](final-web-lint-verified.txt), [types](final-web-types-verified.txt) |
| Frontend production build | Passed. | [Build](final-web-build-verified.txt), [study image build](final-study-build.txt) |
| Real financial and evidence browser journeys | All six passed. | [Real browser log](final-financial-verified.txt) |
| Local isolated acceptance | Financial command, identical replay, and matched reconciliation passed. | [Acceptance](final-acceptance.json) |
| Page accessibility and reflow | All 20 checks passed. | [Page checks](final-web-accessibility-verified.txt) |
| Repaired visual and affected browser checks | All 72 passed with snapshot updates disabled. | [Focused browser log](final-web-browser-verified.txt) |
| Final combined browser suite | All 245 passed. Snapshot updates were disabled. | [Full browser log](final-web-full-verified.txt) |
| Normal workspace | Healthy; financial records unchanged. | [Health and fingerprints](normal-health-final.json) |
| Normal page smoke | All 14 pages rendered. No financial commands were sent. | [Browser record](normal-browser-final.json) |
| Isolated cleanup | No study container, volume, or network remains. Benchmark resources are absent. | [Cleanup](cleanup-final.json) |

The Windows symlink check skipped because the account lacks symlink creation
privilege. The crash helper skips in the parent process; the crash tests execute
it in child processes. No Windows race result is claimed. CGO was disabled.

## Workflow comparison with Step 1

- Account creation, exact-zero opening, freeze, reactivation, and closure passed.
- Funding evidence, approval, posting, and replay passed.
- Transfers retained authentication, CSRF, authorization, exact amounts, and safe retry.
- Lost requests and lost committed responses recovered across clean browser contexts.
- Additive corrections and independent approval passed Go and browser checks.
- Delivery status stayed separate from financial status. Repeated events remained safe.
- Manual reconciliation, exports, account scope, and historical reports remained usable.
- Investigation filters, evidence export, and a second-instance handoff passed.
- Older envelopes and preserved migration histories passed the compatibility checks.

The normal workspace retained six accounts, four transfers, and ten ledger postings.
Its balance, transfer, and posting fingerprints stayed equal. Its migration count
increased from 42 to 45. Two known historical migration records remain preserved.
The optional scheduler is disabled. The normal console does not infer a study replica count.

## Failures found and corrected

Keep the failed runs. Do not replace them with a successful result.

1. A shared reconciliation scanner gained a snapshot field. The export query lacked
   that column. Add the column and verify the exact CSV export contract.
2. A missing replica count decoded as zero. Require the field and bound the full
   response body. Reject stale and oversized observations.
3. Older tests used fixed migration totals and application-clock lease expiry.
   Derive migration expectations from the source. Wait for database lease expiry.
4. The Developer catalogue lacked the new observation route. Add its reviewed
   operation. Keep the static OpenAPI asset bounded at 192 KiB.
5. The scheduler can finish a report after a manual run. Verify the saved manual
   report separately. Accept a later matched report in the latest-result field.
6. Browser assertions read request arrays before dispatch completed. Wait for the
   observed request count. Target the current sign-out alert and test its pending state.
7. One accessibility test scanned all routes within one time limit. Split it by route.
8. Visual fixtures used the previous navy and serif interface, old labels, and a
   moving clock. Compare current source with Step 1. Review representative captures,
   open the required disclosures, preserve permission and mixed-currency checks,
   freeze the fixture clock, and refresh the matching baselines. Keep the original
   failure log and verify again with snapshot updates disabled.
9. Docker Desktop stopped after the normal upgrade. Restart the engine. Verify the
   normal service health and financial fingerprints again.
10. The first cleanup omitted profiled peer and test services. Enable all profiles
    for the exact study project. Verify zero remaining containers, volumes, and networks.

Records: [first Go regression](final-go-run.txt), [first browser regression](final-web-browser.txt),
[first financial run](final-financial-journeys.txt), [observation failure](observation-before.txt),
[visual source comparison](visual-baseline-source-comparison.json), and
[fixture refresh](final-web-fixture-refresh.txt).

## Source and build identity

The baseline revision is `ed1662ebc52fbff49bd75bc957b5d2fa637fa73c`. The workspace
already contained local changes. No Git commit was created by this implementation.

[Baseline identity](baseline.json) and [final application acceptance](final-acceptance.json)
record source fingerprints and image identifiers. Performance records retain the
image identifiers used for measurement. The final observation and export fixes
were built after those measurements. Browser fixture and handover changes followed
the final application build. They do not change financial command code.

[Final workspace identity](final-source.json) records the completed source manifest,
normal runtime image identifiers, and verification results.

## Measured conclusion and limits

All eight workloads posted 300 new transfers. They had no unresolved error,
duplicate journal, or unbalanced journal. Delivery drained and reconciliation matched.
Throughput stayed near 10.2 commands per second under fixed demand. Latency both
improved and increased across configurations. Extra replicas did not prove a
throughput gain at that demand.

The tests used one Docker host. Per-instance limits stayed fixed; total limits grew
with replicas. The runs were short. They do not establish maximum capacity,
multi-machine availability, or production readiness. PostgreSQL remains a shared
dependency. Delivery is at least once. The scheduler can repeat a report after an
unknown result, but it does not repeat financial movement.

The snapshot is database-backed. It is not a Chandy-Lamport global snapshot.
Leader selection is database-coordinated. It is not Bully, Ring, or a consensus
implementation. Kubernetes and other optional syllabus work remain outside scope.

## Handover

Use [the demonstration and recovery instructions](handover.md). Use the
[copyable commit messages](commit-messages.md) to prepare phase commits.
Keep committed financial records during recovery. Prefer an additive repair.
The protected study state files remain ignored. Synthetic runtime volumes were removed.
