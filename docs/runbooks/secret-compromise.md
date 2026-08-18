# Secret compromise or suspected credential exposure

1. Revoke the suspected credential in the secrets manager immediately; record the time, affected scope, and access logs.
2. Rotate the BFF assertion secret, consistency signing key (retaining only the bounded verification overlap), OIDC client secret, and database/Redis credentials as applicable.
3. Expire BFF sessions and invalidate affected workload identities. Do not log the revoked value.
4. Review audit events, private API authentication failures, deployment history, and configuration access.
5. Run authorization and redaction checks before restoring access. Follow [secrets rotation](secrets-rotation.md) for the detailed rotation order.
