# Managed pilot preflight

This gate turns provider screenshots and tickets into one bounded, secret-free,
machine-checked manifest. It does not deploy infrastructure and must not contain
tokens, passwords, connection strings, raw logs, or customer data.

1. Copy `deploy/pilot/pilot-evidence.example.json` into the approved external
   evidence store, not into a commit containing real environment details.
2. Replace every `PENDING` value with an immutable ticket, scan, test, provider,
   or release-evidence reference. Set booleans only after reviewing that source.
3. Before the first restore, validate deployment controls:

   ```text
   go run ./cmd/pilot-preflight --evidence-file <evidence.json>
   ```

4. After the provider-backed isolated restore, require recovery evidence:

   ```text
   go run ./cmd/pilot-preflight --evidence-file <evidence.json> --require-restore
   ```

5. Attach the validator output to the release record. A non-zero exit blocks
   partner traffic. Never edit evidence to fit the validator; correct the
   underlying managed control and rerun its test.

The gate requires HTTPS managed OIDC, the exact tenant/role claims, bounded
session expiry, rotation and denial tests, workload identity, private API/worker/
PostgreSQL/Redis, encrypted continuous backups with at least 35-day retention
and backup age at most 15 minutes, tested alert routes/redaction, zero open
critical/high security findings, and—at release—an isolated zero-mismatch
restore with Redis rebuilt and measured RPO/RTO.
