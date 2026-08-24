# BFF Workload Credential Renewal

The BFF uses two independent proofs: a renewable workload token authenticates the BFF service, and a 60-second actor assertion delegates the signed-in operator context. Neither proof is accepted as the other.

## Production procedure

1. Configure a managed workload identity with only `bff:act-as-user` and private-API network access.
2. Have the provider agent write a short-lived token to the path mounted as `LEDGERSYNC_PRIVATE_API_TOKEN_FILE` using atomic replacement.
3. Do not set `LEDGERSYNC_PRIVATE_API_TOKEN` in production. Request tests must demonstrate that static-token fallback is refused.
4. Alert before token expiry, on renewal failure, and on consecutive private-API `401` responses. Do not log the token or actor assertion.
5. During rotation, keep current and previous actor-signing keys only for the documented overlap. Remove the previous key after the maximum assertion lifetime and review window.

## Failure behavior

- Missing, malformed, expired, or near-expiry workload tokens produce `temporary_unavailable`; the BFF does not call the private API unauthenticated.
- A timed-out transfer response is an unknown outcome. The client retains and reuses the same idempotency key.
- Revocation or suspected compromise requires credential revocation, actor-key rotation, audit review, and tenant-scoped incident handling.
