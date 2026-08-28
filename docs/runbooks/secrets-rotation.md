# Secrets and rotation

LedgerSync uses managed secret references in production. Environment variables are only the delivery mechanism into an isolated workload; `.env` files, image layers, browser bundles, logs, and git must never contain production secrets.

## Secret inventory

- `LEDGERSYNC_DATABASE_URL`: tenant-isolated PostgreSQL credential with TLS required outside local development.
- `LEDGERSYNC_REDIS_ADDR`: private Redis endpoint and separately managed credential when enabled.
- `LEDGERSYNC_CONSISTENCY_SIGNING_KEY`: at least 32 random bytes, independent from every other secret. Maintain the previous key ID during a bounded validation overlap.
- `LEDGERSYNC_SESSION_SECRET` and `LEDGERSYNC_WEB_SESSION_SECRET`: independent 32-byte minimum HMAC secrets; rotation invalidates browser sessions.
- OIDC issuer/audience are configuration, not secret. OIDC private keys stay only with the identity provider.
- `LEDGERSYNC_PRIVATE_API_TOKEN`: short-lived BFF workload OIDC token for the pilot. It must contain only the `bff:act-as-user` scope; the API uses the independently signed, 60-second actor assertion to apply the user’s OIDC subject, tenant, roles, and scopes. Replace static injection with workload OIDC before a shared production launch.

## Local-only workstation material

`scripts/start-local.ps1` generates eight independent 32-byte values for the PostgreSQL owner, the least-privilege API and worker database logins, API session state, consistency proofs, BFF actor assertions, web sessions, and the development private-API credential. They are stored as 64-character hexadecimal values in ignored `data/local-runtime/runtime.env`. On Windows, inherited ACLs are removed and the current user receives the file permission. Values are never printed by the startup, status, bounded-log, backup, or security-evidence commands.

On the first start against an older local stack, the script preserves every existing credential and appends independent API and worker database passwords before the migration job idempotently creates or rotates their LOGIN roles. The owner remains confined to one-shot migration, role provisioning, demo seed, and host recovery. Named-volume data is preserved. If a PostgreSQL volume exists but its matching container is missing, startup fails instead of guessing or resetting data.

Deleting `runtime.env` is a credential-rotation operation, not ordinary cleanup. Stop the stack first and preserve the matching PostgreSQL container so the next start can rotate the database role safely.

## Rotation procedure

1. Create a new secret version in the managed secret store; do not overwrite the active value in place.
2. Deploy workloads that can validate both current and previous consistency key IDs, then make the new key active.
3. Rotate database and workload credentials with a short overlap; verify health, transfer idempotency, and reconciliation before revoking the old credential.
4. Rotate session keys in a maintenance window; revoke existing browser sessions and require sign-in again.
5. Record the action, approver, correlation ID, and outcome in the audit stream. Verify no secret value appears in logs or deployment output.

## Emergency response

Immediately revoke a suspected leaked credential, rotate dependent credentials, invalidate sessions where relevant, inspect audit records, and run reconciliation. A secret incident never justifies disabling idempotency, audit logging, or authorization checks.
