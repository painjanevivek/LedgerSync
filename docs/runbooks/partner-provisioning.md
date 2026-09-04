# Internal design-partner provisioning

Provisioning is a controlled operator workflow, not a public route or self-service admin product. The reviewed JSON creates one tenant, its pilot-currency policy, subject/role mappings, external credential references, zero-opening authorized accounts, and audit evidence in one serializable PostgreSQL transaction. Non-zero value is a separate two-person [opening-import workflow](opening-import.md).

## Preconditions

- jurisdiction, non-custodial positioning, one pilot currency, design-partner contract, and data-retention schedule are approved in writing;
- OIDC/workload credentials already exist in the selected external provider/secret manager;
- the JSON contains only credential references, audiences, scopes, and expiry—never secrets or tokens;
- account UUIDs, external references, categories, subjects, credit/debit rights, velocity limits, and rollback owner are independently reviewed;
- every normal provisioning `opening_minor` is exactly `"0"`;
- the operator uses the least-privilege `ledgersync_provisioning` database role and an approved change UUID.

## Validate and apply

```text
go run ./cmd/provision-partner -action validate -config docs/pilot/provisioning-example.json -pilot-currency INR
go run ./cmd/provision-partner -action apply -config docs/pilot/provisioning-example.json -pilot-currency INR -actor-subject-id <platform-operator> -correlation-id <change-uuid>
```

Archive the printed configuration fingerprint with the reviewed request. A retry with the same correlation ID and identical fingerprint is safe; the same correlation with changed content is rejected.

The decoder rejects unknown fields and trailing JSON. This is intentional: a
reviewer must never approve a limit or permission that the apply workflow would
silently ignore. The supported operator read scopes include `accounts:read`,
`transactions:read`, `transfers:read`, and `reconciliation:read`; write access is
separately gated by `transfers:write` plus account relationships and tenant
policy.

Verify tenant policy, accounts, zero opening balances, owner/credit permissions, subject roles, external credential events, and `partner.provisioned` audit evidence before enabling traffic. If approved opening value is required, keep traffic disabled and follow the opening-import runbook.

## Rollback before financial activity only

```text
go run ./cmd/provision-partner -action rollback -tenant-id <tenant-uuid> -actor-subject-id <different-reviewer> -correlation-id <original-change-uuid>
```

Rollback is refused once any transfer, posted funding, or executed opening import exists. With no financial history it removes access mappings, closes rather than deletes accounts, appends external credential revocation events, and records immutable rollback/audit evidence. It does not delete opening balances, accounts, requests, or evidence.
