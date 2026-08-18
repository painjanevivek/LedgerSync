# Phase 5 — Account authorization and private-boundary evidence

**Date:** 2026-08-18  
**Scope:** User Story 3 — protect accounts and administration

## Delivered controls

- PostgreSQL applies ownership predicates to owned-account, balance, and transaction-history reads. An absent and an inaccessible account use the same safe denial; a client cannot use status codes to enumerate another tenant’s accounts.
- Production API startup requires an OIDC issuer, audience, and 32-byte BFF assertion secret. The maintained `coreos/go-oidc` verifier checks discovery keys, issuer, audience, expiry, and an explicit signing-algorithm allowlist before a principal is accepted.
- The BFF now uses the authorization-code flow with PKCE (`S256`), a signed 10-minute state/nonce/verifier transaction cookie, 5-second discovery/token/JWKS timeouts, and signed ID-token verification before creating its 30-minute HttpOnly session. Browser JavaScript never receives a refresh token or access token.
- Roles and scopes are allowlisted. Read, history, and transfer handlers require their exact scope; unknown claims never add access.
- The BFF issues a 60-second signed actor assertion from its HttpOnly signed session. The private API accepts it only after a separate BFF workload token has passed OIDC verification with `bff:act-as-user`; raw browser actor headers are rejected.
- BFF routes are same-origin, no-store, bounded, session-gated, and CSRF-protected for mutations. Admin routes intentionally resolve to not-found until a privileged, separately audited operator experience is implemented.
- Audit persistence sanitizes metadata by construction; runbooks define what must be recorded and what must never be stored. Secrets use distinct managed values with an explicit rotation procedure.
- Compose keeps data services on a private network, binds only API/web loopback ports, drops capabilities from application containers, enables `no-new-privileges`, and uses read-only filesystems with a small no-exec temporary filesystem.

## Verification

| Check | Result |
|---|---|
| Go focused suite — application/platform/handlers/API/unit tests | Passed |
| OIDC actor assertion signature/tamper test | Passed |
| PostgreSQL live ownership isolation — `go test ./tests/integration -run TestAccount -count=1` | Passed (2.513s) |
| BFF session, CSRF, security-header, and OIDC transaction tests | Passed (4/4) |
| Web lint and production build | Passed |
| Hardened Compose parse (`docker compose ... config --quiet`) | Passed |

## Pilot configuration gate

Before enabling a shared environment, configure the managed OIDC issuer, client ID, client secret where applicable, callback URL, and short-lived BFF workload token containing only `bff:act-as-user`. The current implementation deliberately does not provide a custom login or password lifecycle. Confirm the OIDC provider emits `tenant_id`, `roles`, and `scope` claims, and provision account-owner rows before granting access.
