# BFF Actor Assertion Contract

The private Go API first authenticates the BFF workload credential and requires its `bff:act-as-user` scope. Only then may it accept the separately signed actor assertion in `X-LedgerSync-Actor-Assertion`.

| Claim | Meaning |
|---|---|
| `iss` | Exact configured BFF issuer; default `ledgersync-bff`. |
| `aud` | Exact private API audience; default `ledgersync-private-api`. |
| `kid` | Current or explicitly configured previous signing-key identifier. |
| `jti` | Unique assertion identifier used by the replay guard. |
| `sub` | OIDC-authenticated operator subject. |
| `tenant_id` | Tenant mapping from the BFF session. |
| `roles`, `scopes` | Bounded values; every value must appear in the API allowlist. |
| `iat`, `exp` | Unix seconds. Lifetime is at most 60 seconds with at most 5 seconds of clock skew. |

The HMAC-SHA256 signature covers the decoded UTF-8 JSON payload bytes. The browser never receives the assertion or workload credential. Production obtains the workload token through `LEDGERSYNC_PRIVATE_API_TOKEN_FILE`; a managed workload agent refreshes that file atomically. Static `LEDGERSYNC_PRIVATE_API_TOKEN` is development-only and is refused when the deployment environment is production.

Rotation sequence:

1. Install the new secret as the API's current key while retaining the old key as previous.
2. Switch the BFF key ID and secret to the new current key.
3. Observe assertion failures and allow more than the maximum assertion lifetime plus clock skew.
4. Remove the previous key and record the rotation audit evidence.

The in-process replay guard is shared by all handlers in one API process. A multi-replica production deployment must replace it with the shared replay implementation selected for that environment before scaling beyond one API replica.
