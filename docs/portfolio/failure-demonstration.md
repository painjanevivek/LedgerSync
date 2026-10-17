# Local failure demonstration

Current repair status: [audit repair register](audit-remediation.md). The last verified archived result has seven scenarios. The current source prepares 14 fixed scenarios that cover all 13 required plan rows. The changed runtime route has not been executed with Docker stopped.

Use `node scripts/portfolio/run-demo.mjs --preview` to inspect setup, named tests, fault points, recovery and cleanup. Preview writes no file, invokes no child process and needs no Docker access. The test runs it with child execution and file writes forbidden.

Run `node scripts/portfolio/run-demo.mjs` from the repository. The runner accepts no arbitrary scenario, command, container, endpoint or output argument. It creates a new acceptance project with temporary PostgreSQL and Redis storage, separate loopback ports, fixed CPU/memory limits and synthetic fixtures. It reuses the preserved dependency image IDs. It does not stop or rebuild normal services.

The runner checks exact container IDs, labels, image IDs, declared tmpfs storage and loopback binds before each scenario and before cleanup. It rejects persistent or foreign mounts. Docker can report tmpfs under `HostConfig.Tmpfs` while `Mounts` is empty. Both forms must be consistent with the declared temporary target. A project name alone is not sufficient authorization.

## Historical verified scenarios

| Scenario | Actual check |
|---|---|
| Lost response | Discard the caller's stored result, reopen the service, and retry the same key. One financial effect remains. This is a service-boundary demonstration, not a network-proxy claim. |
| Duplicate command | Concurrent same-key calls return one movement. |
| Duplicate and reordered event | Publish duplicate and older notifications into real isolated Redis. The projection does not apply twice or regress. |
| Stale worker | An expired or superseded claim cannot complete another owner's work. |
| Cache loss | Flush the isolated Redis cache. Read from PostgreSQL and rebuild the projection. |
| Dependency loss | Fail event publication and retain retry state without changing financial records. |
| Process crash | Terminate a real worker child before and after publication. Recover its lease and reconcile. Duplicate delivery is permitted; duplicate money movement is not. |

The complete run has a ten-minute wall bound. Each test has a 150-second timeout and its child process has a separate 180-second bound. Output is capped at 8 MiB per child. Cancellation stops the child process tree and attempts verified cleanup. A host operator can press Ctrl+C or create `cancel.requested` inside that exact run's private bundle. The file only cancels work; it cannot select or start a fault. Use `node scripts/portfolio/test-demo-cancellation.mjs` to verify cancellation in a new disposable project. A test skip, missing named test, malformed output, timeout, or nonzero exit cannot pass. Failed and incomplete records remain. Cleanup failure is a failed check. Never remove a failed bundle to conceal its result.

## Evidence

Bundles stay under ignored `data/portfolio/demo-runs/`. They contain a dirty-source hash inventory, index identity, dependency image IDs, actual applied migration names, migration file hashes, exact test binaries, JSON test output, fixed limits, and cleanup records. Credentials stay in a separate private environment file and are excluded from the artifact list. The artifact verifier streams hashes, checks sizes and path boundaries, rejects symlinks/traversal, and binds source and migration claims to their documents. It rejects contract fixtures as runtime evidence.

Use `node scripts/portfolio/verify-demo.mjs data/portfolio/demo-runs/<exact-run-id>` to verify a bundle. Add `--save-summary` to write a safe read-only summary. Each summary has an immutable run-specific archive. The current summary can change only while the previous archive remains. The browser receives scenario results and provenance hashes. It does not receive credentials, test binaries, private logs or host control.

The Developer page shows the saved result in the advanced Systems Lab disclosure. A saved result is historical. It does not describe current service health or prove a managed-production service level. R11 owns optional-surface availability. The existing disclosure ID and route remain compatible.

## Known local limits

This is one Docker host. It does not prove multi-host partition behavior, cloud recovery or external settlement. Go test binaries, not candidate API images, execute these scenarios. R13 owns exact candidate image qualification. Source/index and normal container identities are checked during a run. The phase separately compares normal financial fingerprints. A later source change makes the old bundle historical; do not relabel it as a test of the new candidate.

## Prepared matrix — runtime repeat pending

The current fixed [scenario registry](../../scripts/portfolio/demo-scenarios.mjs) adds the normal journal/publication route, changed-intent rejection, competing debits, surviving API, scheduler takeover, reconciliation during traffic and tenant denial. The worker crash test covers both before- and after-publication rows. Duplicate/reordered events and dependency loss remain as extra checks. The preview lists every exact test name and matrix mapping.

The new lost-response test uses an owned HTTP server. It commits the real restricted-role command, then aborts its connection before any success header or body. A second owned server recovers the original key and intent. The API failure test first proves that its first server is healthy, stops it, and verifies the surviving server and original receipt. These adapters test the HTTP/commit boundary. They do not qualify a managed load balancer or replace production authentication tests.

New runs record matrix version `r07/v2`. Summary validation requires every current scenario exactly once. Old seven-scenario archives remain valid historical records and cannot claim current full-matrix acceptance. Unknown matrix versions fail closed. A new run must still pass exact-target, cancellation, invariant and cleanup checks before any new summary can be published.
