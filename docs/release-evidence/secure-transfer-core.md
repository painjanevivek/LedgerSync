# Secure transfer core — Phase 7 release evidence

**Recorded:** 2026-08-19 (local engineering verification)  
**Revision:** pending commit on `codex/secure-transfer-foundation`

## What this evidence proves

| Control | Local result |
|---|---|
| Web lint | PASS — `npm run lint` |
| Web unit/security tests | PASS — 6 tests (`npm test`) |
| Production web build | PASS — `npm run build` |
| Browser transfer and accessibility journeys | PASS — 4 Playwright tests in 26.7 s (`npm run test:e2e`) |
| Exact-money property boundary | PASS — Go fuzz, 5-second target with 146,903 executions observed |
| Go unit and transfer-contract paths | PASS — `cmd`, `internal`, `tests/unit`, and `tests/contract` |
| Compose definitions | PASS — regular and Toxiproxy fault profiles validate with `docker compose ... config --quiet` |
| Quickstart container smoke | PASS — all three images build, isolated PostgreSQL/Redis/API/worker/web stack starts, `docker compose exec api migrate` applies migrations, then stack shuts down cleanly |

## Browser journey coverage

1. An authorized operator posts an internal transfer, loses the first response,
   retries, and reuses the identical idempotency key.
2. An amount with unsupported decimal precision is rejected in the UI before an
   API call is made.
3. Insufficient funds says clearly that no money moved.
4. Axe reports no automatically detectable accessibility violations on the
   authenticated overview.

## Release gates intentionally not represented by a local run

The following are external pilot-environment gates, not conditions a local
repository test can manufacture truthfully:

- named pilot jurisdiction and single operational currency;
- real managed-IdP authorization-code-with-PKCE configuration;
- managed PostgreSQL backup age and an isolated provider-backed PITR restore;
- real-pilot reconciliation result of zero mismatches and zero RYEW violations;
- performance measurements at 10, 25, and 50 TPS using dedicated load accounts.

Those gates remain required before any shared production pilot. The release
workflow refuses stale backup, failed restore, or non-zero RYEW evidence when
those values are supplied.
