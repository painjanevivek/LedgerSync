# Validation Quickstart

1. Start the development profile; verify API, worker, web, Postgres and Redis readiness.
2. Sign in as a user with two active same-currency owned test accounts; confirm no other accounts appear.
3. Submit a valid transfer with a new idempotency key; confirm transfer ID, exact balances and one history entry per account.
4. Repeat the same request/key; confirm original response and no additional movement.
5. Run reconciliation; confirm each sampled projection equals net ledger postings.
6. Run concurrent insufficient-total-funds transfers; confirm no overdraft.
7. Induce cache/event delay then transfer/read immediately; confirm completed version or truthful temporary error.
8. Clear Redis or restart worker; confirm primary fallback/recovery and no incorrect balance.
9. Attempt cross-account access and ordinary-user diagnostic access; confirm denial without disclosure.
10. Run isolated restore and reconciliation; preserve evidence.
11. Start the server-gated local demo and verify that its accounts, transfers, and reconciliation records come through the same BFF/API contracts; verify production configuration refuses demo mode.
12. Confirm every overview/account/transfer/reconciliation link opens the selected object and every copy, filter, refresh, menu, review, confirm, and retry control has an observable result.
13. Run the primary journeys at 390×844, 768×1024, 1024×768, 1366×768, 1440×900, and 1920×1080; verify no page-level horizontal overflow and no missing amount, currency, status, identifier, timestamp, tenant context, or evidence provenance.
14. Repeat compact, tablet, and desktop journeys with keyboard-only operation, screen-reader smoke testing, 200% zoom, 400% reflow, increased text spacing, reduced motion, forced colors, slow network, offline transition, and unknown transfer outcome.
15. Verify ledger posting status remains distinct from notification/webhook delivery state and that no aggregate balance combines accounting categories without an approved definition.

Required release evidence: passing migration, unit, integration, contract, E2E, concurrency, fault, accessibility, responsive, visual-regression, frontend-performance and security checks; zero RYEW violations; no unexplained reconciliation mismatch; successful restore drill; demo-isolation proof; signed real-device matrix for iOS, Android, tablet, and desktop/laptop.
