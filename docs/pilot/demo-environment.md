# Isolated operator demo environment

The local Compose profile uses a deterministic demo tenant in PostgreSQL and the same signed session, CSRF, actor assertion, BFF, API authorization, exact-money, and ledger paths as an OIDC-backed operator after identity creation.

## Safety boundary

- Demo mode is enabled only by `LEDGERSYNC_DEMO_MODE=true` on the server.
- `LEDGERSYNC_DEPLOYMENT_ENV=production` rejects all demo identity/seed configuration during server registration.
- The browser cannot select or enable a demo identity.
- The demo tenant and subject are fixed only inside local Compose.
- Demo records are non-production and are labelled in the persistent shell.
- Absence of OIDC or demo configuration renders an authentication-unavailable state with no invented balances.

## Start locally

From the repository root:

```powershell
docker compose -f deploy/compose/docker-compose.yml up --build
```

The `migrate` one-shot service applies versioned schema changes. The `demo-seed` one-shot service then idempotently seeds six categorized USD accounts, posted/rejected transfers, ledger postings, an empty-history account, a frozen account, and matched/mismatch reconciliation evidence. API and web services start only after this sequence succeeds.

Reset only the disposable local environment by explicitly removing the Compose volumes, then starting the stack again. Never reuse the demo tenant, credentials, cookies, or database in a shared or production environment.
