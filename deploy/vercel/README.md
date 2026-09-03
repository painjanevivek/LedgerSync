# LedgerSync on Vercel

LedgerSync must be represented by two separate Vercel projects. Do not import the
repository-root `.env.example` into either project because it is the combined
local Docker-stack template.

## Frontend project: `ledgersync-frontend`

- Connect this repository.
- Set **Root Directory** to `web`.
- Select the **Next.js** framework preset.
- Leave the install, build, and output settings unmodified. [`web/vercel.json`](../../web/vercel.json)
  pins `npm ci`, validates the project root, and runs the Next.js production build.
- Use [`web/.env.example`](../../web/.env.example) only as a variable-name
  checklist; enter real values in Vercel's Environment Variables settings.
- Give Production and Preview different origins, redirect URIs, secrets, and key
  material. Do not expose any secret with a `NEXT_PUBLIC_` prefix.

The current `500 INTERNAL_FUNCTION_INVOCATION_FAILED` deployment cannot be fixed
by environment variables alone. It was built from the repository root, so Vercel
started the long-running Go API as a function. Correct the Root Directory before
redeploying the frontend. Vercel stores Root Directory in the project settings;
it is not a supported `vercel.json` property.

The repository-root [`vercel.json`](../../vercel.json) deliberately fails a
misconfigured root deployment before it can install or build the Go service. A
correctly rooted frontend deployment reads `web/vercel.json` instead.

## Backend project: `ledgersync-backend`

The existing Go API and worker are container-style, long-running processes. A
second Vercel project should be created only after adapters exist for request-based
Functions and scheduled/durable worker execution. The intended backend variable
ownership is documented in [`backend.env.example`](backend.env.example).

The frontend's production BFF also rejects a static private API token and expects
a renewed file-based credential. That sidecar/file mechanism is unavailable in a
standard Vercel Function. Before connecting the two production projects, implement
a Vercel-compatible workload identity or managed short-lived credential provider.

## Values shared across the two projects

Only the BFF assertion contract is deliberately shared:

- `LEDGERSYNC_BFF_ASSERTION_SECRET`
- `LEDGERSYNC_BFF_ASSERTION_KEY_ID`
- `LEDGERSYNC_BFF_ASSERTION_ISSUER`
- `LEDGERSYNC_BFF_ASSERTION_AUDIENCE`
- the two `PREVIOUS_*` values temporarily used during key rotation

The frontend signs assertions and the backend verifies them, so both sides must
use matching values. All database, Redis, webhook, backend session, browser
session, and OIDC client-secret values stay only in their owning project.

## Safe update sequence

1. Correct the frontend project's Root Directory to `web`.
2. Leave Framework Preset as Next.js and clear any dashboard overrides for the
   install, build, and output settings so `web/vercel.json` remains authoritative.
3. Delete the combined variables that do not appear in `web/.env.example`.
4. Enter frontend Production values, then separate Preview values.
5. Redeploy and verify the public page and authentication callback.
6. Implement and test the Vercel backend adapters and credential strategy.
7. Create the backend project, add only backend variables, and deploy it.
8. Set the frontend private API URL to the backend domain, align BFF keys, and run
   authenticated end-to-end verification.

## Local validation

Run the configuration check from the repository root:

```sh
node deploy/vercel/validate-config.mjs
```

Run the same root guard and production build used by Vercel from `web`:

```sh
node scripts/verify-vercel-root.mjs
npm run build
```

The build may use disposable placeholder values for required server-only settings.
Do not copy production secrets into the repository or pass them on a command line.
