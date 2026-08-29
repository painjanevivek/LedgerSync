# Provider-led outbound payouts

**Status:** Proposed engineering feature; live-provider activation remains externally gated.
**Scope:** INR-only, provider-led B2B outbound payouts.
**Authority:** `docs/plans/provider-led-payout-production-program.md`.

## User scenarios

### US1 — Request a protected payout (P1)

An authorized operator selects an eligible source account and beneficiary,
reviews the exact INR amount and provider fee, and submits one payout request.
The system reserves available value immediately, records a stable payout ID, and
does not yet create a posted journal.

**Acceptance:** repeated requests using the same idempotency key return the
same payout; concurrent requests cannot reserve more than available balance.

### US2 — Approve and send a payout (P1)

A different, recently authenticated operator approves an eligible request. The
system sends exactly one provider instruction asynchronously and exposes a
truthful pending state while waiting for the provider result.

**Acceptance:** the requester cannot approve; expired or stale approvals never
dispatch; a lost client response does not duplicate the provider command.

### US3 — Settle or release reserved value (P1)

The provider reports settlement, rejection, cancellation, or expiry. LedgerSync
posts a balanced journal only for authenticated provider-confirmed settlement;
all non-settled terminal paths release the reservation without editing history.

**Acceptance:** settlement posts source → external-payout clearing and exact
source → fee account entries. Failure preserves ledger truth and restores only
the reserved spendable amount.

### US4 — Investigate and reconcile a payout (P2)

Finance and operations can inspect the request, reservation, approvals,
provider attempt, callback, exact fee, clearing postings, settlement record,
and safe next action.

**Acceptance:** a missing, duplicate, reordered, or mismatching provider record
is visible as unresolved evidence; it never changes a balance by inference.

## Functional requirements

- FR-001: Model payouts separately from internal transfers.
- FR-002: A payout uses exact INR minor units, an authorized source, an
  approved beneficiary reference, a finite approved limit, and an idempotency key.
- FR-003: Before approval, atomically reserve source available balance without
  creating journal postings.
- FR-004: Requester and approver must be different subjects. Approval, dispatch,
  and sensitive cancellation require recent authentication evidence.
- FR-005: Provider instructions use durable work and an adapter-neutral
  idempotency reference. No external request runs inside a financial transaction.
- FR-006: Only a verified provider outcome may settle. Settlement posts balanced
  immutable entries to the finance-approved clearing and fee accounts.
- FR-007: Fee amount, currency, source, and policy version are explicit before
  approval; ambiguous or changed fees are rejected.
- FR-008: Provider callbacks require signature, timestamp, nonce, and replay
  verification. Provider event evidence is immutable.
- FR-009: Failure, cancellation, expiry, duplicate callbacks, and timeout
  recovery preserve history and release an active reservation exactly once.
- FR-010: Reconciliation compares provider settlement records with LedgerSync
  payout records and produces immutable mismatch evidence.
- FR-011: Operators can list and inspect payouts with pagination, filters,
  permission, loading, empty, error, and safe-next-action states.

## State model

`requested → reserved → pending_approval → approved → dispatching → provider_pending → settled | failed | cancelled | expired → reconciled`

`reconciled` is an evidence result after a terminal provider outcome. It never
implies that an unresolved payout was settled.

## Non-goals

- Direct custody, bank credentials, inbound collection, cards, FX,
  cross-border settlement, consumer wallets, or public self-service.
- A live provider adapter before legal, provider, finance, security, and
  operations gates are approved.

## Dependencies and assumptions

- A licensed provider is selected and contracted before its live adapter is
  enabled. The repository first uses a deterministic fake sandbox adapter.
- Finance provides tenant-specific clearing and fee accounts, limits, retention,
  and fee policy before activation.
- The first release uses two-person approval and charges an explicit provider
  fee to the source account by default.

## Success criteria

- Success, duplicate, timeout, callback replay, rejection, expiry,
  insufficient-funds, and settlement-mismatch scenarios preserve exact,
  explainable balances.
- Operators can identify the requested, approved, dispatched, provider, and
  ledger outcome without database access.
- A partner cannot cause a financial posting merely by calling an API;
  settlement requires authentic provider evidence.
