# Phase 4 clean full-stack runtime evidence

## Claim boundary

This phase proves the repository's disposable local production-shaped assembly. It does not claim a managed cloud, real IdP, provider PITR, production secrets, external network policy, or design-partner environment is configured.

## Required evidence

| Check | Pass condition |
|---|---|
| Clean build/start | PostgreSQL and Redis healthy; migration/seed exit 0; API, worker, and web healthy |
| Migration ownership | Migration job applies all 11 migrations; restart run is idempotent; API never migrates |
| Authorized account evidence | BFF session lists owned accounts and opens object-specific detail |
| Exact movement | Source balance decreases by exactly one integer minor unit |
| Immediate visibility | Requirement-bearing balance read returns the new authoritative amount |
| Retry safety | Same payload/key returns one transfer ID and does not decrease balance again |
| Seed replay safety | Re-running the demo seed cannot overwrite a committed projection/version or opening balance |
| Immutable detail | Transfer detail reports posted financial state and its journal record |
| Service recovery | Web, API, worker, and Redis restart; a new exact/retry journey passes |
| Database recovery | PostgreSQL restarts; readiness, migrations, and a new exact/retry journey pass |
| Reconciliation | New authoritative run is `matched` with string mismatch count `0` |
| Runtime isolation | Workloads are non-root/read-only; PostgreSQL, Redis, and API are not host-published; only the BFF is exposed |

## Reproducible commands

Follow `docs/runbooks/local-runtime-smoke.md`. CI performs the same sequence in the `real-stack` job and uploads bounded logs only on failure. Use a distinct `LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY` for each intentional new movement; a retry within one test always reuses that exact key.

## Local execution record

Executed on 2026-08-24 (Asia/Calcutta) from the Phase 3 baseline `3aec14a` plus the Phase 4 working tree:

- Clean build/start completed with 11 migrations; migration and seed jobs exited successfully and all five long-running services became healthy.
- Four distinct exact-transfer journeys passed across clean startup, service/Redis restart, PostgreSQL restart, and a final private-runtime rebuild. Each journey proved a new request, same-key replay, one stable transfer ID, one exact minor-unit debit, matching committed/visible balance, and immutable detail.
- A deliberate seed rerun left the source projection amount, ledger amount, version, and opening balance unchanged. This closed a discovered defect where the previous upsert could rewind financial state.
- PostgreSQL restart, migration re-entry, API recovery, and a new transfer journey passed without reseeding or data loss.
- Reconciliation run `551e4dca-fbec-4e49-a557-dde2a139caab` completed `matched` across six accounts with mismatch count `0`.
- API, worker, and web ran as `ledgersync` with read-only root filesystems. PostgreSQL, Redis, and API had no host bindings; only web was exposed on `127.0.0.1:3000`.
- Compose and GitHub Actions YAML parsed successfully. The final disposable-project cleanup result is recorded by the phase handoff.

Secrets, cookies, response bodies, and unbounded logs were not captured.
