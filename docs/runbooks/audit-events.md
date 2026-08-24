# Audit events

LedgerSync writes a durable audit event for security-sensitive operational actions. Financial transfer completion is already recorded atomically in the transfer, journal, ledger postings, outbox records, and transfer audit event; it is not reconstructed from application logs.

## Required event shape

- `tenant_id`, actor subject (when known), event type, target type, outcome, correlation ID, and UTC occurrence time.
- Target IDs are identifiers only. Store no account balances, minor-unit amounts, request body, bearer token, session value, email, address, or customer-supplied free text.
- `sanitized_metadata` accepts a small allowlisted diagnostic context such as `reason=authorization_denied` or `source=operator_console`. Sensitive keys and oversized values are discarded by the repository.

## Policy

- Record success and denial for operator security settings, access-role changes, reconciliation acknowledgement, export initiation, and break-glass use.
- API scope denials and transactional transfer-policy denials are recorded through the same redacting repository. Reconciliation completion is committed with its run and mismatch evidence. Dead-work replay and partner provisioning add their own correlated events when those controlled operations execute.
- The actor must be an OIDC subject or a documented system workload identity. Never accept a browser-provided actor ID.
- Audit-write failure blocks a sensitive state-changing administrative operation. It must not make a completed financial transfer appear unsuccessful; transfer auditing remains in that transfer transaction.
- PostgreSQL rejects update/delete of audit and reconciliation rows. Corrections are new correlated events; existing evidence is never rewritten.
- Retain audit records according to the pilot jurisdiction’s signed retention policy. Access is restricted to the `audit:read` scope and is itself audited.

## Investigation

1. Begin with the correlation ID from the customer-visible error or operator action.
2. Query only the affected tenant’s audit stream; do not run cross-tenant support queries without break-glass approval.
3. Correlate with immutable transfer/journal IDs, never mutable cache data.
4. Export redacted evidence through the approved incident channel; do not paste raw logs into tickets.

## Privileged operational evidence

After an approved configuration change, credential rotation, restore drill, or break-glass action, run `go run ./cmd/audit-event` through the temporarily authorized database identity. Supply the tenant, trusted actor subject, approved event type, target category/ID, outcome, correlation UUID, and ticket reference. The command rejects unrecognized event types and the repository removes sensitive metadata. Revoke the temporary grant immediately after verifying the appended row.
