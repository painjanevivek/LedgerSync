# Phase 8 — reconciliation and support evidence

## Authoritative behavior

- List and detail data comes from tenant-scoped PostgreSQL reconciliation records through the private API and same-origin BFF.
- `Passed` requires a completed `matched` run with zero mismatches, a run ID, and completion time.
- Missing/unreachable evidence renders `Evidence unavailable`; it never renders a synthetic pass.
- Run detail preserves scope, account/posting counts, mismatch count, ledger watermark, application version, correlation ID, and UTC timestamps.
- Mismatch results remain financial failures even when an operator later acknowledges or investigates them; no balance-edit/delete control exists.

## Transfer support chain

Transfer detail exposes immutable transfer facts, journal ID, debit/credit postings, and separate delivery status. `Posted + delivery delayed` explicitly means the ledger movement is complete while downstream publication still needs attention.

## Access and responsive behavior

- Transfer and reconciliation IDs are authorized inside the API; inaccessible IDs return safe not-found/denial responses.
- Full identifiers remain copyable.
- Current evidence precedes history on compact screens.
- Desktop comparison tables become prioritized evidence cards on compact screens without changing financial meaning.
