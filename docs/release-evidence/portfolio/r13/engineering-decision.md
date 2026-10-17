# R13 local engineering decision

Decision date: 16 October 2026, Asia/Calcutta. Reviewer: the local engineering author. No independent reviewer is claimed.

## Candidate

The runtime candidate uses main `d300036fdefca6abb1725cd12c9dbc52e8029f63`. Its frozen source manifest SHA-256 is `8d48338b81c560380795ff0d6e55abfd9153e70e5630cec03d0d12af7c212b00`. The manifest records every source file and the preserved local media separately. Normal named volumes were excluded from all test targets. The candidate ran schema 000064 with separate restricted API and worker logins.

The private candidate bundle is `data/portfolio/r13-20261016/candidate-20261015203518-a616e0b7`. The completion record hashes the result and selected artifacts. Private credentials, database contents and raw scanner matches are not publication inputs.

## Gate mapping

| Gate | Evidence and scope |
|---|---|
| Financial and access invariants | Exact-main Quality gates run 38800424907: formatting, vet, static analysis, Linux race detector, money/model fuzz, critical coverage, actual-role matrix, complete integration/fault suites, web checks and generated-artifact drift. Named optional host profiles remain separately identified local evidence. |
| Contracts and production path | Exact-main Contract validation 38800424711 and Production-path CI 38800424827 passed. Local candidate also passed six actual BFF/API browser journeys, the Go system suite, and two original-key replay calls after API restart. |
| Browser safety and accessibility | Recorded full Windows suite and reviewed visual baselines, Linux CI browser/performance checks, then six final real-stack journeys. No physical-device or independent assistive-technology claim. |
| Model | Default independent corpus in exact-main integration; separately recorded 256-operation extended local run. No exhaustive schedule proof. |
| Optional capability sets | R11 optional profile and later R13 repeat retain their source identities. Core and separate supported sets pass; invalid combinations remain rejected. |
| Recovery and populated upgrade | R13 synthetic recovery evidence plus the consolidation restore of the exact preserved normal backup into a disposable target. Migration 000064, exact fingerprints, inert policy-role bootstrap, unsafe-role rejection, and cleanup passed. These runs do not establish managed recovery or current backup coverage for later writes. |
| Security | Exact-main Supply-chain gates 38800424893 passed. Final local network-disabled history scan inspected 356 commits with no leaks after 105 exact reviewed fingerprints. All 56 source findings match previously reviewed noncredential contexts. Four final image vulnerability commands passed at HIGH/CRITICAL and four CycloneDX SBOM commands passed. Advisory cache was updated 15 October and next update is 16 October 13:13 UTC. Development-only braces risk remains in qualification.md. |
| Build and operating boundary | Actual Docker context export passed: 916 files, 4,686,125 bytes, no private runtime state. Image IDs, users, read-only filesystems, dropped capabilities, temporary storage, loopback bindings, resource samples and exact cleanup are recorded. |
| Capacity | The fresh five-minute selected component workload and recovery result is recorded separately. Historical R09 three-run results retain their own binaries. No HTTP, multi-host, hold-capacity or SLA claim. |
| Preservation | Current normal schema is 000064 with 66 retained migration records, including two known legacy records. Eight accounts, five transfers, 20 postings and all three original financial fingerprints must match at completion. The earlier authorized main update deliberately upgraded the normal schema; R13 qualification does not repeat that upgrade. |

## Permitted use and remaining duties

Approve the identified candidate only for local portfolio review after every referenced gate is confirmed in the completion record. Core includes accounts, controlled funding, transfers, recovery, corrections, internal reconciliation, authorized evidence/export, delivery, diagnostics and developer contracts. Approved opening imports are a separate provisioning-only operation. They are not external statement reconciliation.

R15 and R16 are selected extensions. Their new schema and financial behavior need their own tests and a final integrated repeat. R17 prepares a deployment package locally. Managed identity, network behavior, backup/failover, operator ownership and independent human reviews remain unexecuted. A source-manifest review candidate is not a signed committed release. Repeat required committed-release gates after the authorized final commit and push.
