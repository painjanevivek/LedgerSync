# Operator UI completion baseline

**Captured:** 2026-08-23  
**Git baseline:** `15d6853` on `main` with preserved in-progress UI/design changes  
**Status:** Baseline accepted for implementation; not a production release result

## What already passes

- `go test ./...`: passed. The Go telemetry uploader could not create its local token in the restricted environment, but the test process completed successfully.
- `npm run lint`: passed.
- `npm test`: 6 tests passed after allowing Node test-worker process creation.
- `npm run build`: passed with Next.js 16.3.1; account, transfer, reconciliation, session, and BFF routes compiled.

## Known product gaps at baseline

- Unauthenticated routes render fictional account, transfer, tenant, operator, and reconciliation data from React constants.
- The core transfer form is disabled in the preview state.
- Record actions do not consistently open object-specific detail views.
- Reconciliation preview claims a passing result without an authorized evidence response.
- Several copy, refresh, workspace, and row affordances are visual-only or mislabeled.
- The current shell and data density are not proven across the required mobile/tablet/laptop/desktop matrix.
- Existing automated accessibility checks do not yet prove focus, reflow, forced-colors, long-content, virtual-keyboard, or real-device behavior.

## Captured evidence

- `docs/design/audit/placeholder-functionality/01-overview.png`
- `docs/design/audit/placeholder-functionality/02-accounts.png`
- `docs/design/audit/placeholder-functionality/03-transfers.png`
- `docs/design/audit/placeholder-functionality/04-reconciliation.png`
- `docs/design/qa/overview-reference-vs-implementation.png`

## Baseline preservation rules

- Do not delete or rewrite unrelated user/untracked files.
- Replace fictional route behavior only after real demo/API coverage exists.
- Preserve exact-money, idempotency, authorization, RYEW, reconciliation, and restore evidence while changing UI behavior.
- Each completion phase must update its own evidence rather than overwriting this baseline.

