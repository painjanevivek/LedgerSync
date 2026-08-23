# Financial UI semantics

Status: engineering baseline approved for the isolated pilot demo; finance owner approval remains required before external production traffic.

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
