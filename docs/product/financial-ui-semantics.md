# Financial UI semantics

Status: engineering baseline approved for the isolated pilot demo; finance owner approval remains required before external production traffic.

## Approval register

| Decision | Engineering proposal | Accountable approval | Status |
|---|---|---|---|
| Pilot currency and exponent | One configured ISO currency; formatting exponent follows the approved currency metadata | Finance + product | Pending |
| `Available balance` definition | Authoritative PostgreSQL projection at the shown version/time; never a stale fallback labelled current | Finance | Pending |
| Operating-controlled membership | `operating`, `payroll`, `payables`, `expenses`, `reserve`; customer funds excluded | Finance + legal/custody owner | Pending per partner |
| Posted/rejected/unknown language | Use the definitions below without collapsing outcomes | Finance + support | Pending |
| Reconciliation pass claim | Persisted completed `matched` run with zero mismatches and evidence identifiers | Finance + operations | Pending |
| Timestamp policy | Render UTC explicitly; do not imply a local accounting date | Finance + support | Pending |

No pending row may be converted to “approved” without a named reviewer, UTC
date, decision-ticket/evidence reference, and any partner-specific exception.

## Balance presentation

- `Available balance` is the authoritative PostgreSQL projection returned with currency, balance version, and `as_of` time.
- A failed authoritative refresh removes the current-value claim; an older value is not relabelled as current.
- Cross-currency totals are prohibited.
- `customer_funds` is always shown separately from operating-controlled categories. It must not be presented as operating capital.
- The demo overview may aggregate same-currency `operating`, `payroll`, `payables`, `expenses`, and `reserve` accounts under the explicit label `Operating-controlled balances`. A finance owner must approve or narrow this grouping for each production design partner.

## Transfer and delivery language

- `Posted` means the immutable double-entry journal and balance projection committed.
- `Rejected` means no ledger posting occurred and no money moved.
- `Result not confirmed` means the client cannot yet prove the outcome. The only safe retry is the same request with the same idempotency key.
- `Delivery delayed` describes downstream outbox/webhook publication only. It never reverses or weakens a posted financial result.

## Reconciliation language

- `Passed` is allowed only when a tenant-authorized, completed persisted run has status `matched`, zero mismatches, a run ID, and a completion time.
- `Mismatch detected`, `Failed`, `Running`, and `Evidence unavailable` are distinct states.
- Absence of evidence is never displayed as zero mismatches or as a passing control.

## Identifiers and time

- Account, transfer, journal, posting, run, and correlation identifiers are immutable evidence and remain copyable in full.
- Visual shortening may aid scanning but must preserve a full title/reveal/copy path.
- All operator-console timestamps are rendered explicitly in UTC.
