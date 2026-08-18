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

Required release evidence: passing migration, unit, integration, contract, E2E, concurrency, fault, accessibility and security checks; zero RYEW violations; no reconciliation mismatch; successful restore drill.
