# Local operator workspace

The local Compose profile starts as a clean operator workspace. It creates only the local tenant, operator roles, and required policies; it does not create accounts, balances, journals, transfers, or reconciliation evidence. The browser uses the same signed session, CSRF, actor assertion, BFF, API authorization, exact-money, and ledger paths as an OIDC-backed operator after identity creation.

## Safety boundary

- One-click local login is enabled only when `LEDGERSYNC_LOCAL_LOGIN_ENABLED=true`, the application is running in development, and the request comes from a loopback host.
- `LEDGERSYNC_DEPLOYMENT_ENV=production` rejects local identity configuration during server registration.
- The browser cannot select or forge a local identity. The server creates a short-lived signed session for the fixed local operator.
- A signed-out browser renders only the login layer and no financial evidence.
- The clean workspace never invents balances, reconciliation results, transfers, or delivery outcomes.
- OIDC remains the production authentication path.

## Start locally

From the repository root:

```powershell
docker compose -f deploy/compose/docker-compose.yml up --build
```

The `migrate` one-shot service applies versioned schema changes. The initialization service then applies `local-bootstrap.sql`, which idempotently creates the local identity boundary and policies without financial records. API and web services start only after this sequence succeeds.

To replace an older demo-initialized volume with a clean workspace, run:

```powershell
.\scripts\reset-local.ps1 -Confirmation 'DELETE LEDGERSYNC LOCAL DATA' -InitializationMode empty
.\scripts\start-local.ps1
```

This reset deletes the disposable local database volume. Never use the local tenant, credentials, cookies, or database in a shared or production environment.

Legacy deterministic fixtures remain available only when explicitly requested for the retry/fault lab. They are not part of the normal product path or a fresh local start.
