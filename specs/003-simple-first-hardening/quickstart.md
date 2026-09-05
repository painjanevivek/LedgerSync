# Local Verification

1. Start the existing local dependencies and Go API without changing secrets.
2. Start `web` with its documented local environment.
3. Verify Simple view first, then switch to Expert view from the profile control.
4. Exercise Home, Accounts, Add money, Transfers, Tasks, and one unknown-outcome flow.
5. Run frontend lint, unit/security/UI tests, production build, and Playwright.
6. Run Go unit, integration, migration compatibility, and race checks.
