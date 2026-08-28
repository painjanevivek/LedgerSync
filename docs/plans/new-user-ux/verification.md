# New-user UX verification

Date: 2026-08-29

## Automated checks

- `npm run lint` — passed.
- `npm test` — passed: 98 tests.
- `npm run build` — passed.
- `npx playwright test tests/e2e/responsive.spec.ts tests/e2e/accessibility.spec.ts --workers=1` — passed: 24 tests.
- `npm run test:visual:update -- --workers=1` — passed: 33 tests; accepted updated baselines only after manual visual inspection.
- The normal visual run passed 32 of 33 checks; one 0.01% timestamp-only mismatch in the mixed-currency overview passed when retried in isolation. The console’s existing timer behavior is deliberately unchanged.

## Visual review

- Reviewed the populated Accounts desktop screen: optional search and filters are clearly marked; table and primary action remain readable.
- Reviewed the Transfers desktop screen: required account and amount fields are clearly marked; the contextual control panel remains separate from the one dominant action.
- Updated desktop, compact, error, empty, offline, intake, review, and read-only baselines that changed solely because of the new shared labels and simpler headings.

## Scope confirmation

- New-workspace guidance links only to implemented tasks: create an account, add a funding record, review it, and make a transfer.
- Requirement markers describe server-enforced validation; optional filters and references remain optional.
- This evidence validates the local operator console. It does not represent a production deployment or independent production authentication.
