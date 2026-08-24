# Local MVP Phase 1 — runtime and scope evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows/Docker Desktop workstation

**Evidence binding:** the Git commit containing this document

## What was proved

The supported `compose` project recovered from its previously partial state without deleting or replacing the existing PostgreSQL/Redis named volumes. The dashboard became ready only after PostgreSQL, Redis, schema migration, replay-safe demo seed, API, worker, web health, and real BFF reads all passed.

## Reproducible checks

| Check | Result |
|---|---|
| `docker compose -p compose -f deploy/compose/docker-compose.yml config -q` | Passed |
| PowerShell parser over all six local runtime scripts | Passed with zero syntax errors |
| Startup retry/config/API/worker Go tests | Passed |
| Full non-race Go suite across commands, internals, unit, contract, integration, fault, and system packages | Passed |
| Web lint, 20 unit tests, TypeScript, and production build | Passed |
| Full build through `scripts/start-local.ps1` | Passed; URL printed only after BFF smoke |
| Individual Redis, PostgreSQL, API, worker, and web restart plus `start-local -SkipBuild` | Five of five passed |
| API/worker start during temporary PostgreSQL unavailability | Recovered inside bounded startup window |
| API/worker start during temporary Redis unavailability | Recovered, including an observed Redis `LOADING` retry |
| `scripts/stop-local.ps1` followed by `scripts/start-local.ps1 -SkipBuild` | Passed; volumes preserved |
| Wrong destructive-reset confirmation | Correctly refused with a non-zero exit |
| Real BFF idempotency system test | First response `201`, same-key replay `201`, replay flag changed from false to true, no second movement |
| Real BFF reconciliation evidence test | Passed |

Bounded startup logs recorded dependency, retry attempt, transient category, next delay, and remaining startup time without credentials or raw balances.

The Windows host has `CGO_ENABLED=0`, so `go test -race` is unavailable locally. This is not silently waived: the pushed commit remains gated by the repository's Linux CI race job.

## Financial-state preservation

Before individual service restart testing, the existing database reported:

- 12 migration rows, latest `000012_transfer_velocity_capacity.up.sql`;
- 6 accounts, 6 projections, and 6 opening-balance evidence rows;
- 140,583 transfer rows, 140,582 journals, and 281,164 postings;
- 0 unpublished outbox events.

Those values were identical after all five service restarts. The real-stack idempotency proof then intentionally added exactly one transfer, one journal, and two postings. After the later full stop/start, the database reported 140,584 transfers, 140,583 journals, 281,166 postings, unchanged schema/account/projection/opening-evidence counts, and 0 unpublished outbox events. No seed replay or lifecycle action rewound financial state.

## Network boundary

Compose inspection showed only `web` published to the host as `127.0.0.1:3000->3000/tcp`. PostgreSQL, Redis, and API ports remained container-private; the worker published no port.

## Scope decision

This pass applies only to the one-workstation, loopback-only INR demo product. Managed OIDC, cloud deployment, provider PITR, legal approvals, partner onboarding, and physical-device sign-off remain unchanged in the production-pilot gate register and are not represented as completed.
