# LedgerSync on Vercel

LedgerSync deploys from one repository as two independent Vercel projects. The
frontend project builds only `web`; the backend project builds the root Go API
and invokes its bounded background drain once per minute.

## 1. Frontend project: `ledgersync-frontend`

- Connect this repository and set **Root Directory** to `web`.
- Select the **Next.js** framework preset.
- Clear dashboard install/build/output overrides so `web/vercel.json` owns them.
- Add the variables listed in `web/.env.example` separately for Preview and
  Production. Never expose a secret with a `NEXT_PUBLIC_` prefix.
- Set `LEDGERSYNC_PRIVATE_API_URL` to the backend project's stable URL.
- Configure the private API OAuth client variables. The BFF exchanges them for
  short-lived workload access tokens and caches each token only until shortly
  before expiry.

The frontend build guard fails if Vercel accidentally runs it outside `web`.

## 2. Backend project: `ledgersync-backend`

- Connect the same repository and keep **Root Directory** at the repository root.
- Select the **Go** framework preset.
- Clear dashboard build/output overrides so the root `vercel.json` is authoritative.
- Add only the variables in `deploy/vercel/backend.env.example`.
- Use provider-issued pooled PostgreSQL and `rediss://` Redis URLs. The runtime
  accepts both managed Redis URLs and the local `host:port` form.
- Create a high-entropy `CRON_SECRET` in Vercel. Vercel sends it as a Bearer
  credential to `GET /internal/cron/drain`; the endpoint rejects missing or incorrect
  credentials using a constant-time comparison.
- Do not set `PORT` or `LEDGERSYNC_HTTP_ADDR`. Vercel injects `PORT`, and the Go
  server gives it precedence automatically.

The root build produces `server` from `cmd/api`. The existing Go router remains
the HTTP entrypoint. The scheduled drain reuses the same durable outbox,
webhook, verification, and balance-projection workers, runs bounded batches for
at most 50 seconds, and exits before the next minute's invocation.

Minute-level cron requires a paid Vercel plan. If near-real-time 200 ms worker
latency is required, deploy `cmd/outbox-worker` as a continuously running
container instead and remove the root `crons` entry.

## 3. Shared contracts

Only these BFF assertion values are deliberately shared between projects:

- `LEDGERSYNC_BFF_ASSERTION_SECRET`
- `LEDGERSYNC_BFF_ASSERTION_KEY_ID`
- `LEDGERSYNC_BFF_ASSERTION_ISSUER`
- `LEDGERSYNC_BFF_ASSERTION_AUDIENCE`
- the two `PREVIOUS_*` values temporarily used during key rotation

The frontend signs actor assertions and the backend verifies them. The OAuth
workload client must also be registered in the backend's
`LEDGERSYNC_OIDC_CLIENT_TENANT_MAP` and receive the backend resource audience.
Database, Redis, webhook, browser-session, and OIDC client secrets stay only in
their owning project.

## 4. Data and release sequence

1. Provision isolated Preview and Production PostgreSQL and Redis resources.
2. Run `cmd/migrate` against the target database from a controlled release job.
   Never run migrations in a Vercel build or application cold start.
3. Deploy the backend Preview project and verify `/healthz` and `/readyz`.
4. Invoke `/internal/cron/drain` manually with its Preview cron credential and confirm
   outbox work drains without exposing the credential in logs.
5. Deploy the frontend Preview project, configure its OAuth workload client and
   backend URL, then exercise authenticated reads and an idempotent test transfer.
6. Repeat with Production-scoped resources and promote the exact verified builds.

Preview must never point to the Production database, Redis instance, backend, or
identity credentials.

## 5. Known storage boundary

Recovery evidence currently uses `LEDGERSYNC_RECOVERY_EVIDENCE_ROOT`. Vercel's
local filesystem is not durable storage. Routes that create or depend on durable
recovery evidence must remain disabled operationally until that evidence is moved
to encrypted object storage or PostgreSQL. Do not represent `/tmp` as durable
recovery evidence.

## Local validation

From the repository root:

```sh
node deploy/vercel/validate-config.mjs
go test ./...
go build -o server ./cmd/api
```

From `web`:

```sh
npm ci
npm run lint
npm test
npm run build
```

The builds may use disposable placeholder values where server-only settings are
validated at build time. Never add production secrets to files or command lines.
