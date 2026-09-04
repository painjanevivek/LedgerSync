# Controlled opening import

Use `cmd/opening-import` only to establish independently evidenced opening
value for newly provisioned, zero-balance customer accounts. It is an offline
finance operation, not a public API, funding shortcut, settlement claim, or
way to edit accepted ledger history.

## Preconditions

- Partner provisioning completed with every account at an exact zero opening.
- The manifest contains 1–10,000 unique account UUIDs in one approved pilot
  currency and positive canonical integer minor-unit values.
- The requester and approver are distinct tenant subjects with the `finance`
  role and strong authenticated identities.
- The command uses a login inheriting only `ledgersync_provisioning` and a new
  correlation UUID for each request, approval, and execution action.
- Traffic, transfers, and funding remain disabled until execution and
  reconciliation finish.

## Validate and request

```powershell
go run ./cmd/opening-import -action validate `
  -manifest docs/pilot/opening-import-example.json -pilot-currency INR

go run ./cmd/opening-import -action request `
  -manifest docs/pilot/opening-import-example.json -pilot-currency INR `
  -actor-subject-id <finance-requester> -correlation-id <request-uuid>
```

Archive the printed SHA-256, row count, total minor units, reviewed manifest,
and change record outside the source tree. Do not approve a hash that differs
from the request evidence.

## Independently approve and execute

```powershell
go run ./cmd/opening-import -action approve `
  -manifest docs/pilot/opening-import-example.json -pilot-currency INR `
  -actor-subject-id <different-finance-approver> -correlation-id <approval-uuid>

go run ./cmd/opening-import -action execute `
  -manifest docs/pilot/opening-import-example.json -pilot-currency INR `
  -actor-subject-id <finance-executor> -correlation-id <execution-uuid>
```

The database re-hashes the immutable stored rows, reconciles currency, row
count, and total, locks every account in stable order, and requires all opening
and projected balances to remain zero with no postings. It then updates every
baseline/projection, writes one execution record, emits account-balance events,
and appends requester/approver/executor audit evidence in one transaction.

Retries of the exact batch are safe. Changed content, self-approval, partial
account drift, direct manifest edits, a second execution, and calls from API or
support roles are rejected. Once execution succeeds it is irreversible history:
correct an error through an approved compensating workflow, never SQL updates,
deletion, or a down migration.
