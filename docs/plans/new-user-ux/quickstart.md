# Quickstart: verify the new-user UX

## Prerequisites

- Start the supported local LedgerSync stack.
- Open the local console and sign in through the local login flow.

## Checks

1. Open the dashboard with a new local workspace.
   - Expected: the next step is clear and no fake balance or approval is shown.
2. Open each form.
   - Expected: every field is marked `Required` or `Optional` according to the field inventory.
3. Submit an empty required form.
   - Expected: the error says what to add and focus remains usable.
4. Complete account setup, funding record entry, review, and transfer preparation.
   - Expected: labels and guidance use plain language; existing server checks still work.
5. Open advanced pages.
   - Expected: technical explanation is available but does not hide important financial facts.
6. Run the quality checks from `web/`:

```powershell
npm run lint
npm test
npm run build
npx playwright test tests/e2e/responsive.spec.ts --workers=1
npx playwright test tests/e2e/accessibility.spec.ts --workers=1
npm run test:visual -- --workers=1
```

7. Inspect the changed visual screenshots before accepting them.
