# ADR 0007: Opaque, revocable BFF sessions

## Status

Accepted.

## Context

The browser previously carried signed identity, authorization, CSRF, and consistency state. Although signed, that payload could approach cookie-size limits, could not be revoked immediately, and retained stolen-session validity until expiry.

## Decision

- The browser receives a random 256-bit opaque session handle only.
- PostgreSQL stores only the SHA-256 digest of that handle together with tenant, operator, authorization, CSRF, consistency requirements, authentication time, expiry, and revocation time.
- The BFF creates, resolves, rotates, revokes, and updates sessions through private workload-authenticated API routes.
- Sign-in and step-up rotate the previous handle. Sign-out revokes the server record before clearing the browser cookie.
- Production uses a `__Host-` session cookie. OIDC transactions use a `__Secure-` cookie with the same explicit attributes for creation and deletion.
- Session resolution and storage failures fail closed. Expired and revoked sessions are indistinguishable from unknown handles to the browser.
- A bounded retention method deletes expired/revoked records without scanning unbounded history.

The signed session codec remains an internal, server-to-server request envelope while existing BFF route authorization is migrated. It is never stored in the browser.

## Consequences

Sessions are immediately revocable and no longer constrained by browser cookie payload size. Each authenticated request adds a private session lookup, so database availability and indexing are part of the login availability boundary. No deployment resource or secret is provisioned by this decision.
